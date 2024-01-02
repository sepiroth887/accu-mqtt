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
)

type Start struct {
	BrokerURL    string  `help:"MQTT server url (mqtt://foo.bar:1883)" short:"b" env:"ACCU_MQTT_BROKER"`
	AccuAPIToken string  `help:"Accu Weather API app token" short:"t" env:"ACCU_MQTT_API_TOKEN"`
	Latitude     float32 `help:"Latitude of location with up to 3 digit precision, e.g. -120.223" short:"x" env:"ACCU_MQTT_LATITUDE"`
	Longitude    float32 `help:"Longitude of location with up to 3 digit precision, e.g. -120.223" short:"y" env:"ACCU_MQTT_LONGITUDE"`
	UseTestData  bool    `help:"Use sample ./response.json instead of real API (for testing)" short:"d" env:"ACCU_MQTT_TEST_DATA"`
}

var cli struct {
	Debug bool  `help:"enable debug" short:"v"`
	Start Start `cmd:"" help:"start the provider"`
}

var mqttClient mqtt.Client

func registerSensors(debug bool) error {
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
	if debug {
		fmt.Printf("Sending registration payload:\n%v\n", registerRain)
	}

	if t := mqttClient.Publish("homeassistant/sensor/accu-mqtt/rain/config", 0, false, data); t.Wait() && t.Error() != nil {
		return t.Error()
	}

	registerMinutesSensor := Registration{
		Name:                "Rain Start",
		UniqueID:            "a63ca366-9eda-4301-9428-93b173d15b9a_rain",
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
	data, _ = json.Marshal(&registerMinutesSensor)
	if debug {
		fmt.Printf("Sending registration payload:\n%v\n", registerMinutesSensor)
	}

	if t := mqttClient.Publish("homeassistant/sensor/accu-mqtt/rainstart/config", 0, false, data); t.Wait() && t.Error() != nil {
		return t.Error()
	}

	return nil
}

func (s *Start) Run(debug bool) error {
	opts := mqtt.NewClientOptions()
	opts.AddBroker(s.BrokerURL)
	opts.SetClientID("accu-mqtt")
	opts.SetCleanSession(true)
	opts.SetStore(mqtt.NewMemoryStore())

	mqttClient = mqtt.NewClient(opts)

	if t := mqttClient.Connect(); t.Wait() && t.Error() != nil {
		return t.Error()
	}
	defer mqttClient.Disconnect(100)

	err := registerSensors(debug)
	if err != nil {
		return err
	}

	loc := fmt.Sprintf("%.3f,%.3f", s.Latitude, s.Longitude)
	apiKey := s.AccuAPIToken
	cast, err := loadCast(s.UseTestData, loc, apiKey)
	if err != nil {
		fmt.Println("Failed to retrieve MinuteCast: ", err)
	}
	if debug {
		fmt.Printf("Retrieved cast with data:\n%v\n", cast)
	}

	if debug {
		fmt.Println("Sending online status payload on accu-mqtt/available")
	}
	if t := mqttClient.Publish("accu-mqtt/available", 0, true, "online"); t.Wait() && t.Error() != nil {
		return t.Error()
	}

	var data []byte
	go func() {
		for range time.NewTicker(time.Second * 10).C {
			state := getStateFromCast(cast)
			data, _ = json.Marshal(&state)
			if debug {
				fmt.Printf("Sending state update:\n%v\n", state)
			}
			if t := mqttClient.Publish("accu-mqtt/attributes", 0, false, data); t.Wait() && t.Error() != nil {
				fmt.Println("failed to publish state: ", t.Error())
			}
			if t := mqttClient.Publish("accu-mqtt/state", 0, false, data); t.Wait() && t.Error() != nil {
				fmt.Println("failed to publish state: ", t.Error())
			}
			if time.Now().After(cast.UpdateTime.Add(time.Hour * 1)) {
				if debug {
					fmt.Println("updating cast")
				}
				cast, err = loadCast(s.UseTestData, loc, apiKey)
				if err != nil {
					fmt.Println("Failed to retrieve MinuteCast: ", err)
				}
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

func main() {
	ctx := kong.Parse(&cli)
	ctx.Run(cli.Debug)
}

func loadCast(useTestData bool, loc string, apiKey string) (cast MinuteCast, err error) {
	hClient := http.Client{}
	hClient.Timeout = time.Second * 15
	cast = MinuteCast{
		UpdateTime: time.Now(),
	}

	if !useTestData {
		res, err := hClient.Get(fmt.Sprintf("https://dataservice.accuweather.com/forecasts/v1/minute?q=%s&apikey=%s", loc, apiKey))
		if err != nil {
			return cast, err
		}
		defer res.Body.Close()
		data, err := io.ReadAll(res.Body)
		if err != nil {
			return cast, err
		}
		err = json.Unmarshal(data, &cast)
		if err != nil {
			return cast, err
		}

		os.WriteFile("./last_update.json", data, 0777)
	} else {
		data, _ := os.ReadFile("./last_update.json")
		err := json.Unmarshal(data, &cast)
		if err != nil {
			return cast, err
		}
	}
	return
}

func getStateFromCast(cast MinuteCast) State {
	if cast.Summary.Type == nil {
		return State{
			Weather:   "CLEAR",
			RainStart: 0,
			Message:   cast.Summary.Phrase,
		}
	}

	rainStart := 0
	switch *cast.Summary.Type {
	case "RAIN":
		for _, sum := range cast.Summaries {
			if sum.Type != nil && isRainingNow(cast.UpdateTime, sum.StartMinute, sum.EndMinute) {
				return State{Weather: "RAIN", RainStart: 0, Message: cast.Summary.Phrase}
			}
			if sum.Type != nil && rainStart == 0 {
				rainStart = sum.StartMinute
			}
		}
		return State{Weather: "SOON", RainStart: rainStart - (int(time.Since(cast.UpdateTime).Minutes())), Message: cast.Summary.Phrase}
	default:
		return State{Weather: "No Data", RainStart: 0, Message: "unknown cast type: " + *cast.Summary.Type}
	}
}

func isRainingNow(lastUpdate time.Time, startMin, endMin int) bool {
	return time.Now().After(lastUpdate.Add(time.Duration(startMin)*time.Minute)) && time.Now().Before(lastUpdate.Add(time.Duration(endMin)*time.Minute))
}
