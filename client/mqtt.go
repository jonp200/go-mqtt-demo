// Copyright 2025 Jon Perada. All rights reserved.
// Use of this source code is governed by an MIT-style
// license that can be found in the LICENSE file.

package client

import (
	"fmt"
	"os"

	mqtt "github.com/eclipse/paho.mqtt.golang"
	glog "github.com/labstack/gommon/log"
)

type Mqtt struct {
	mqtt.Client
	done chan struct{}
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

	return &Mqtt{Client: mqtt.NewClient(opts), done: make(chan struct{})}, nil
}

func (m *Mqtt) Disconnect() {
	glog.Infof("disconnecting mqtt client...")

	close(m.done)

	if m.Client.IsConnected() {
		m.Client.Disconnect(DefaultQuiesceTimeout)
	}

	glog.Infof("mqtt client disconnected")
}
