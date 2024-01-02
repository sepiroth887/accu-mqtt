package main

import "time"

type MinuteCast struct {
	Summary struct {
		Phrase string  `json:"Phrase"`
		Type   *string `json:"Type"`
		TypeID int     `json:"TypeId"`
	} `json:"Summary"`
	Summaries  []Summary `json:"Summaries"`
	Link       string    `json:"Link"`
	MobileLink string    `json:"MobileLink"`
	UpdateTime time.Time
}

type Summary struct {
	StartMinute int     `json:"StartMinute"`
	EndMinute   int     `json:"EndMinute"`
	CountMinute int     `json:"CountMinute"`
	MinuteText  string  `json:"MinuteText"`
	Type        *string `json:"Type"`
	TypeID      int     `json:"TypeId"`
}

type Registration struct {
	Name                   string `json:"name"`
	UniqueID               string `json:"unique_id"`
	StateTopic             string `json:"state_topic"`
	ValueTemplate          string `json:"value_template"`
	JSONAttributesTopic    string `json:"json_attributes_topic"`
	JSONAttributesTemplate string `json:"json_attributes_template"`
	Icon                   string `json:"icon"`
	AvailabilityTopic      string `json:"availability_topic"`
	PayloadAvailable       string `json:"payload_available"`
	PayloadNotAvailable    string `json:"payload_not_available"`
	Device                 Device `json:"device"`
	Platform               string `json:"platform"`
}

type Device struct {
	Identifiers  []string `json:"identifiers"`
	Name         string   `json:"name"`
	Manufacturer string   `json:"manufacturer"`
	Model        string   `json:"model"`
}

type State struct {
	Weather   string `json:"weather"`
	RainStart int    `json:"rain_start"`
	RainEnd   int    `json:"rain_end"`
	Message   string `json:"message"`
}
