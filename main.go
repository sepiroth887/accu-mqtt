package main

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/signal"
	"time"

	"github.com/alecthomas/kong"
	mqtt "github.com/eclipse/paho.mqtt.golang"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
)

type CommandError struct {
	error
	exitCode int
}

func (c CommandError) GetExitCode() int {
	return c.exitCode
}

func NewCommandError(err error, exitCode int) CommandError {
	c := CommandError{}
	c.error = err
	c.exitCode = exitCode
	return c
}

type Start struct {
	BrokerURL    string  `help:"MQTT server url (mqtt://foo.bar:1883)" short:"b" env:"ACCU_MQTT_BROKER"`
	AccuAPIToken string  `help:"Accu Weather API app token" short:"t" env:"ACCU_MQTT_API_TOKEN"`
	Latitude     float32 `help:"Latitude of location with up to 3 digit precision, e.g. -120.223" short:"x" env:"ACCU_MQTT_LATITUDE"`
	Longitude    float32 `help:"Longitude of location with up to 3 digit precision, e.g. -120.223" short:"y" env:"ACCU_MQTT_LONGITUDE"`
	UseTestData  bool    `help:"Use sample ./response.json instead of real API (for testing)" short:"d" env:"ACCU_MQTT_TEST_DATA" default:"false"`
}

type cli struct {
	Debug      bool  `help:"enable debug" short:"v"`
	Start      Start `cmd:"" help:"start the provider"`
	mqttClient mqtt.Client
	httpClient http.Client
	cast       MinuteCast
}

var mqttClient mqtt.Client

func main() {
	zerolog.TimeFieldFormat = zerolog.TimeFormatUnix
	c := cli{}
	ctx := kong.Parse(&c)
	if err := ctx.Run(c); err != nil {
		fmt.Printf("failed to run command: %v\n", err)
		os.Exit(err.(CommandError).GetExitCode())
	}
}

func (c *cli) RefreshCast(apiKey, loc string) {
	if c.Start.UseTestData {
		return
	}
	for range time.NewTicker(time.Minute * 80).C {
		cast, err := queryAPI(c.httpClient, apiKey, loc)
		if err != nil {
			log.Warn().Err(err)
		}
		c.cast = cast
	}
}

func queryAPI(httpClient http.Client, apiKey, loc string) (MinuteCast, error) {
	res, err := httpClient.Get(fmt.Sprintf("https://dataservice.accuweather.com/forecasts/v1/minute?q=%s&apikey=%s", loc, apiKey))
	if err != nil {
		log.Warn().Err(err)
		return MinuteCast{}, err
	}

	if res.StatusCode < 199 || res.StatusCode > 299 {
		data, _ := io.ReadAll(res.Body)
		return MinuteCast{}, fmt.Errorf("failed to request cast from live api: status [%d]: %s", res.StatusCode, string(data))
	}

	defer res.Body.Close()
	data, err := io.ReadAll(res.Body)
	if err != nil {
		return MinuteCast{}, err
	}

	var cast MinuteCast
	err = json.Unmarshal(data, &cast)
	if err != nil {
		return MinuteCast{}, err
	}
	cast.UpdateTime = time.Now()
	log.Debug().Msgf("Received live cast: %s", string(data))

	data, _ = json.Marshal(&cast)
	os.WriteFile("./last_update.json", data, 0777)

	return cast, err
}

func (s *Start) Run(c cli) error {
	opts := mqtt.NewClientOptions()
	opts.AddBroker(s.BrokerURL)
	opts.SetClientID("accu-mqtt")
	opts.SetCleanSession(true)
	opts.SetStore(mqtt.NewMemoryStore())

	zerolog.SetGlobalLevel(zerolog.InfoLevel)
	if c.Debug {
		zerolog.SetGlobalLevel(zerolog.DebugLevel)
	}
	mqttClient = mqtt.NewClient(opts)
	c.mqttClient = mqttClient
	c.httpClient = http.Client{
		Timeout: 10 * time.Second,
	}

	if t := mqttClient.Connect(); t.Wait() && t.Error() != nil {
		return NewCommandError(t.Error(), 1)
	}
	defer mqttClient.Disconnect(100)

	err := c.registerSensors()
	if err != nil {
		return NewCommandError(err, 2)
	}

	loc := fmt.Sprintf("%.3f,%.3f", s.Latitude, s.Longitude)
	apiKey := s.AccuAPIToken

	cast, err := c.loadCast()
	if cast.UpdateTime.Before(time.Now().Add(time.Minute * 80)) {
		cast, err = queryAPI(c.httpClient, apiKey, loc)
		if err != nil {
			log.Warn().Err(err)
		}
	}
	go c.RefreshCast(apiKey, loc)

	if err != nil {
		fmt.Println("Failed to retrieve MinuteCast: ", err)
	}
	if c.Debug {
		fmt.Printf("Retrieved cast with data:\n%v\n", cast)
	}

	if c.Debug {
		fmt.Println("Sending online status payload on accu-mqtt/available")
	}
	if t := mqttClient.Publish("accu-mqtt/available", 0, true, "online"); t.Wait() && t.Error() != nil {
		return NewCommandError(t.Error(), 3)
	}

	var data []byte
	go func() {
		for range time.NewTicker(time.Second * 30).C {
			state := getStateFromCast(cast)
			data, _ = json.Marshal(&state)
			log.Debug().Msgf("Sending state update:\n%v\n", state)

			if t := mqttClient.Publish("accu-mqtt/attributes", 0, false, data); t.Wait() && t.Error() != nil {
				log.Warn().Msgf("failed to publish state: %v", t.Error())
			}
			if t := mqttClient.Publish("accu-mqtt/state", 0, false, data); t.Wait() && t.Error() != nil {
				log.Warn().Msgf("failed to publish state: %v", t.Error())
			}

			if t := mqttClient.Publish("accu-mqtt/available", 0, true, "online"); t.Wait() && t.Error() != nil {
				log.Warn().Msgf("failed to publish state: %v", t.Error())
			}
		}
	}()

	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, os.Interrupt)

	<-sigCh

	if t := mqttClient.Publish("accu-mqtt/available", 0, true, "offline"); t.Wait() && t.Error() != nil {
		fmt.Println(t.Error())
	}

	return nil
}

func (c *cli) registerSensors() error {
	registerRain := Registration{
		Name:                "Rain Indicator",
		UniqueID:            "a63ca366-9eda-4301-9428-93b173d15b9a_accu",
		StateTopic:          "accu-mqtt/state",
		Icon:                "mdi:information-outline",
		Platform:            "mqtt",
		AvailabilityTopic:   "accu-mqtt/available",
		ValueTemplate:       "{{ value_json.weather }}",
		JSONAttributesTopic: "accu-mqtt/state",
		PayloadAvailable:    "online",
		PayloadNotAvailable: "offline",
		Device: Device{
			Identifiers:  []string{"a63ca366-9eda-4301-9428-93b173d15b9a"},
			Name:         "Accu Weather MinuteCast",
			Manufacturer: "me",
			Model:        "t2000",
		},
	}
	data, _ := json.Marshal(&registerRain)
	log.Debug().Msgf("Sending registration payload:\n%v\n", registerRain)

	if t := mqttClient.Publish("homeassistant/sensor/accu-mqtt/rain/config", 0, false, data); t.Wait() && t.Error() != nil {
		return t.Error()
	}

	registerStartSensor := Registration{
		Name:                "Rain Start",
		UniqueID:            "a63ca366-9eda-4301-9428-93b173d15b9a_rainstart",
		StateTopic:          "accu-mqtt/state",
		Icon:                "mdi:information-outline",
		Platform:            "mqtt",
		AvailabilityTopic:   "accu-mqtt/available",
		ValueTemplate:       "{{ value_json.rain_start }}",
		JSONAttributesTopic: "accu-mqtt/state",
		PayloadAvailable:    "online",
		PayloadNotAvailable: "offline",
		Device: Device{
			Identifiers:  []string{"a63ca366-9eda-4301-9428-93b173d15b9a"},
			Name:         "Accu Weather MinuteCast",
			Manufacturer: "me",
			Model:        "t2000",
		},
	}
	data, _ = json.Marshal(&registerStartSensor)

	log.Debug().Msgf("Sending registration payload:\n%v\n", registerStartSensor)

	if t := mqttClient.Publish("homeassistant/sensor/accu-mqtt/rainstart/config", 0, false, data); t.Wait() && t.Error() != nil {
		return t.Error()
	}

	registerEndSensor := Registration{
		Name:                "Rain End",
		UniqueID:            "a63ca366-9eda-4301-9428-93b173d15b9a_rainend",
		StateTopic:          "accu-mqtt/state",
		Icon:                "mdi:information-outline",
		Platform:            "mqtt",
		AvailabilityTopic:   "accu-mqtt/available",
		ValueTemplate:       "{{ value_json.rain_end }}",
		JSONAttributesTopic: "accu-mqtt/state",
		PayloadAvailable:    "online",
		PayloadNotAvailable: "offline",
		Device: Device{
			Identifiers:  []string{"a63ca366-9eda-4301-9428-93b173d15b9a"},
			Name:         "Accu Weather MinuteCast",
			Manufacturer: "me",
			Model:        "t2000",
		},
	}
	data, _ = json.Marshal(&registerEndSensor)
	log.Debug().Msgf("Sending registration payload:\n%v\n", registerEndSensor)

	if t := mqttClient.Publish("homeassistant/sensor/accu-mqtt/rainend/config", 0, false, data); t.Wait() && t.Error() != nil {
		return t.Error()
	}

	return nil
}

func (c *cli) loadCast() (cast MinuteCast, err error) {
	data, _ := os.ReadFile("./last_update.json")
	err = json.Unmarshal(data, &cast)
	return
}

func getStateFromCast(cast MinuteCast) State {
	if cast.Summary.Type == nil {
		return State{
			Weather: "CLEAR",
			Message: cast.Summary.Phrase,
		}
	}

	weather := "CLEAR"
	rainStart := 0
	rainEnd := 0
	switch *cast.Summary.Type {
	case "RAIN":
		for _, sum := range cast.Summaries {
			if sum.Type != nil && isRainingNow(cast.UpdateTime, sum.StartMinute, sum.EndMinute) {
				return State{
					Weather:   "RAIN",
					RainStart: 0,
					RainEnd:   sum.EndMinute - (int(time.Since(cast.UpdateTime).Minutes())),
					Message:   cast.Summary.Phrase,
				}
			}
			if sum.Type != nil && rainStart == 0 && !time.Now().After(cast.UpdateTime.Add(time.Duration(sum.StartMinute)*time.Minute)) {
				rainStart = sum.StartMinute
				rainEnd = sum.EndMinute
				weather = "SOON"
			}
		}
		return State{
			Weather:   weather,
			RainStart: max(rainStart-(int(time.Since(cast.UpdateTime).Minutes())), 0),
			RainEnd:   max(rainEnd-(int(time.Since(cast.UpdateTime).Minutes())), 0),
			Message:   cast.Summary.Phrase,
		}
	default:
		return State{
			Weather: "No Data",
			Message: "unknown cast type: " + *cast.Summary.Type,
		}
	}
}

func isRainingNow(lastUpdate time.Time, startMin, endMin int) bool {
	return time.Now().After(lastUpdate.Add(time.Duration(startMin)*time.Minute)) && time.Now().Before(lastUpdate.Add(time.Duration(endMin)*time.Minute))
}
