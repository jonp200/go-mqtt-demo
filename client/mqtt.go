package client

import (
	"fmt"
	"os"

	mqtt "github.com/eclipse/paho.mqtt.golang"
)

type Mqtt struct {
	mqtt.Client
}

func NewMqtt(caName, clientId string) (*Mqtt, error) {
	opts := mqtt.NewClientOptions().
		AddBroker(fmt.Sprintf("mqtts://%v:%v", os.Getenv("BROKER_ADDRESS"), os.Getenv("BROKER_PORT"))).
		SetDefaultPublishHandler(defaultPublishHandler)

	if _, err := setTLSConfig(opts, caName); err != nil {
		return nil, err
	}

	setAuth(clientId, opts)
	setCleanSession(opts)
	setReconnect(opts)

	opts.OnConnectAttempt = onConnectAttempt
	opts.OnReconnecting = onReconnecting
	opts.OnConnect = onConnect
	opts.OnConnectionLost = onConnectionLost

	return &Mqtt{mqtt.NewClient(opts)}, nil
}
