package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"log/slog"
	"os"
	"os/signal"
	"syscall"
	"time"

	MQTT "github.com/eclipse/paho.mqtt.golang"
)

type Message struct {
	topic   string
	message []byte
}

type Pc321State struct {
	VoltageL1           *uint32 `json:"101"`
	CurrentL1           *uint32 `json:"102"`
	ActivePowerL1       *int32  `json:"103"`
	PowerFactorL1       *uint32 `json:"104"`
	EnergyConsumptionL1 *uint32 `json:"106"`

	VoltageL2           *uint32 `json:"111"`
	CurrentL2           *uint32 `json:"112"`
	ActivePowerL2       *int32  `json:"113"`
	PowerFactorL2       *uint32 `json:"114"`
	EnergyConsumptionL2 *uint32 `json:"116"`

	VoltageL3           *uint32 `json:"121"`
	CurrentL3           *uint32 `json:"122"`
	ActivePowerL3       *int32  `json:"123"`
	PowerFactorL3       *uint32 `json:"124"`
	EnergyConsumptionL3 *uint32 `json:"126"`

	TotalEnergyConsumption *uint32 `json:"131"`
	TotalCurrent           *uint32 `json:"132"`
	TotalActivePower       *int32  `json:"133"`

	Frequency         *uint32 `json:"135"`
	Temperature       *uint32 `json:"136"`
	DeviceStatus      *uint8  `json:"137"`
	PhaseSeqDetection *uint8  `json:"138"`
}

type RoundedFloat float64

func (r RoundedFloat) MarshalJSON() ([]byte, error) {
	s := fmt.Sprintf("%.*f", 3, r)
	return []byte(s), nil
}

func main() {
	topic := flag.String("topic", "", "The topic name to/from which to publish/subscribe")
	broker := flag.String("broker", "", "The broker URI. ex: tcp://10.10.1.1:1883")
	password := flag.String("password", "", "The password (optional)")
	user := flag.String("user", "", "The User (optional)")
	id := flag.String("id", "testgoid", "The ClientID (optional)")
	cleansess := flag.Bool("clean", false, "Set Clean Session (default false)")
	store := flag.String("store", ":memory:", "The Store Directory (default use memory store)")
	flag.Parse()

	if *topic == "" {
		slog.Error("Invalid setting for -topic, must not be empty")
		return
	}

	opts := MQTT.NewClientOptions()
	opts.AddBroker(*broker)
	opts.SetClientID(*id)
	opts.SetUsername(*user)
	opts.SetPassword(*password)
	opts.SetCleanSession(*cleansess)
	if *store != ":memory:" {
		opts.SetStore(MQTT.NewFileStore(*store))
	}

	choke := make(chan Message)

	opts.SetDefaultPublishHandler(func(client MQTT.Client, msg MQTT.Message) {
		choke <- Message{msg.Topic(), msg.Payload()}
	})

	client := MQTT.NewClient(opts)
	token := client.Connect()
	if !token.WaitTimeout(5 * time.Second) {
		slog.Error("Connect timeout")
		os.Exit(1)
	}
	if token.Error() != nil {
		slog.Error("Connect failed", "err", token.Error())
		os.Exit(1)
	}

	c := make(chan os.Signal, 1)
	signal.Notify(c, os.Interrupt, syscall.SIGTERM)
	go func() {
		<-c

		client.Disconnect(250)
		slog.Info("Subscriber Disconnected")
		os.Exit(1)
	}()

	publishConfig(client)

	token = client.Subscribe(*topic, 0, nil)
	if !token.WaitTimeout(5 * time.Second) {
		slog.Error("Subscribe timeout")
		os.Exit(1)
	}
	if token.Error() != nil {
		slog.Error("Subscription failed", "err", token.Error())
		os.Exit(1)
	}

	for {
		incoming := <-choke
		slog.Debug("Received message", "topic", incoming.topic, "message", string(incoming.message))

		var data Pc321State
		err := json.Unmarshal(incoming.message, &data)
		if err != nil {
			slog.Error("Failed to unmarshal message", "error", err)
			continue
		}

		payload := make(map[string]RoundedFloat)
		// Range: 0-5000,              Unit: 0.1V
		if data.VoltageL1 != nil {
			payload["voltage_l1"] = RoundedFloat(*data.VoltageL1) * 0.1
		}
		if data.VoltageL2 != nil {
			payload["voltage_l2"] = RoundedFloat(*data.VoltageL2) * 0.1
		}
		if data.VoltageL3 != nil {
			payload["voltage_l3"] = RoundedFloat(*data.VoltageL3) * 0.1
		}
		// Range: 0-3000000,           Unit: mA
		if data.CurrentL1 != nil {
			payload["current_l1"] = RoundedFloat(*data.CurrentL1) * 0.001
		}
		if data.CurrentL2 != nil {
			payload["current_l2"] = RoundedFloat(*data.CurrentL2) * 0.001
		}
		if data.CurrentL3 != nil {
			payload["current_l3"] = RoundedFloat(*data.CurrentL3) * 0.001
		}
		// Range: -6600000-6600000,    Unit: W
		if data.ActivePowerL1 != nil {
			payload["power_l1"] = RoundedFloat(*data.ActivePowerL1)
		}
		if data.ActivePowerL2 != nil {
			payload["power_l2"] = RoundedFloat(*data.ActivePowerL2)
		}
		if data.ActivePowerL3 != nil {
			payload["power_l3"] = RoundedFloat(*data.ActivePowerL3)
		}
		// Range: 0-100,               Unit: 0.01
		if data.PowerFactorL1 != nil {
			payload["power_factor_l1"] = RoundedFloat(*data.PowerFactorL1) * 0.01
		}
		if data.PowerFactorL2 != nil {
			payload["power_factor_l2"] = RoundedFloat(*data.PowerFactorL2) * 0.01
		}
		if data.PowerFactorL3 != nil {
			payload["power_factor_l3"] = RoundedFloat(*data.PowerFactorL3) * 0.01
		}
		// Range: 0-2000000000,        Unit: 0.01kW·h
		// Looks like documentation is wrong, it should be 0.001kW·h
		if data.EnergyConsumptionL1 != nil {
			payload["energy_l1"] = RoundedFloat(*data.EnergyConsumptionL1) * 0.001
		}
		if data.EnergyConsumptionL2 != nil {
			payload["energy_l2"] = RoundedFloat(*data.EnergyConsumptionL2) * 0.001
		}
		if data.EnergyConsumptionL3 != nil {
			payload["energy_l3"] = RoundedFloat(*data.EnergyConsumptionL3) * 0.001
		}
		// Range: 0-2000000000,        Unit: 0.01kW·h
		if data.TotalEnergyConsumption != nil {
			payload["energy"] = RoundedFloat(*data.TotalEnergyConsumption) * 0.001
		}
		// Range: 0-9000000,           Unit: mA
		if data.TotalCurrent != nil {
			payload["current"] = RoundedFloat(*data.TotalCurrent) * 0.001
		}
		// Range: -19800000-19800000,  Unit: W
		if data.TotalActivePower != nil {
			payload["power"] = RoundedFloat(*data.TotalActivePower)
		}
		// Range: 0-80,                Unit: Hz
		if data.Frequency != nil {
			payload["frequency"] = RoundedFloat(*data.Frequency)
		}
		// Range: -100-800,            Unit: 0.1℃
		if data.Temperature != nil {
			payload["temperature"] = RoundedFloat(*data.Temperature) * 0.1
		}

		jsonPayload, err := json.Marshal(payload)
		if err != nil {
			slog.Error("Failed to marshal payload", "error", err)
			continue
		}

		slog.Info("Publishing message", "topic", "pc321", "message", string(jsonPayload))
		token := client.Publish("smacker/pc321", 0, false, string(jsonPayload))
		if !token.WaitTimeout(5 * time.Second) {
			slog.Error("Publish timeout")
		}
		if token.Error() != nil {
			slog.Error("Failed to publish message", "error", token.Error())
		}
	}

}

var deviceJson = `{"identifiers": ["smacker_pc321"],"manufacturer": "Owon","model": "3-Phase clamp power meter","name": "Energy Meter"}`

func publishConfigMetric(client MQTT.Client, metric, payload string) {
	token := client.Publish(
		fmt.Sprintf("homeassistant/sensor/pc321/%s/config", metric), 0, true,
		fmt.Sprintf(`{"device": %s, %s}`, deviceJson, payload))
	if !token.WaitTimeout(5 * time.Second) {
		slog.Error("Publish timeout")
	}
	if token.Error() != nil {
		slog.Error("Failed to publish message", "error", token.Error())
	}
}

func publishConfig(client MQTT.Client) {
	energyPayload := `"device_class": "energy","enabled_by_default": true,"object_id": "pc321_energy%s","state_class": "total_increasing","state_topic": "smacker/pc321","unique_id": "pc321_energy%s","unit_of_measurement": "kWh","value_template": "{{ value_json.energy%s }}"`
	publishConfigMetric(client, "energy", fmt.Sprintf(energyPayload, "", "", ""))
	publishConfigMetric(client, "energy_l1", fmt.Sprintf(energyPayload, "_l1", "_l1", "_l1"))
	publishConfigMetric(client, "energy_l2", fmt.Sprintf(energyPayload, "_l2", "_l2", "_l2"))
	publishConfigMetric(client, "energy_l3", fmt.Sprintf(energyPayload, "_l3", "_l3", "_l3"))

	powerPayload := `"device_class": "power","enabled_by_default": true,"entity_category": "diagnostic","object_id": "pc321_power%s","state_class": "measurement","state_topic": "smacker/pc321","unique_id": "pc321_power%s","unit_of_measurement": "W","value_template": "{{ value_json.power%s }}"`
	publishConfigMetric(client, "power", fmt.Sprintf(powerPayload, "", "", ""))
	publishConfigMetric(client, "power_l1", fmt.Sprintf(powerPayload, "_l1", "_l1", "_l1"))
	publishConfigMetric(client, "power_l2", fmt.Sprintf(powerPayload, "_l2", "_l2", "_l2"))
	publishConfigMetric(client, "power_l3", fmt.Sprintf(powerPayload, "_l3", "_l3", "_l3"))

	// TODO: publish other metrics too
}
