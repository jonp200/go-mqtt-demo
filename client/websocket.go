package client

import (
	"fmt"
	"os"
	"sync/atomic"

	mqtt "github.com/eclipse/paho.mqtt.golang"
	glog "github.com/labstack/gommon/log"
)

type WebSocket struct {
	mqtt.Client
	*ConnEventWatcher
}

// WsQos is the QoS when subscribing. Preferred to use QoS level 1 when subscribing.
const WsQos = 1

func NewWebSocket(caName, clientId, topic string) (*WebSocket, error) {
	opts := mqtt.NewClientOptions().
		AddBroker(fmt.Sprintf("wss://%v:%v/mqtt", os.Getenv("BROKER_ADDRESS"), os.Getenv("BROKER_WS_PORT")))

	if _, err := setTLSConfig(opts, caName); err != nil {
		return nil, err
	}

	setAuth(clientId, opts)
	setCleanSession(opts)
	setReconnect(opts)

	opts.OnConnectAttempt = onConnectAttempt
	opts.OnReconnecting = onReconnecting

	watcher := NewConnEventWatcher()
	opts.OnConnect = func(client mqtt.Client) {
		var offlineId atomic.Int64

		init := true
		token := client.Subscribe(
			topic, WsQos, func(_ mqtt.Client, msg mqtt.Message) {
				relayMessage(watcher, msg, &init, &offlineId)
			},
		)

		if err := token.Error(); token.Wait() && err != nil {
			glog.Errorf("Subscribe error: %v", err)
		}

		glog.Infof("Connected to broker over WebSocket")

		init = false // init is complete and succeeding messages should be sent to the online message chan
	}

	opts.OnConnectionLost = onConnectionLost

	return &WebSocket{Client: mqtt.NewClient(opts), ConnEventWatcher: watcher}, nil
}

func relayMessage(watcher *ConnEventWatcher, msg mqtt.Message, init *bool, offlineId *atomic.Int64) {
	// Send messages found upon init to the offline message chan
	if *init {
		// MessageID() is always 0 and cannot be used as an ID. Maybe there's a config necessary?
		offlineId.Add(1)

		watcher.OfflineMessage <- OfflineMessage{
			Id:      offlineId.Load(),
			Payload: msg.Payload(),
		}
		glog.Infof(
			"Message [%v] (while offline) from topic: %v\n>>\t%s", offlineId.Load(), msg.Topic(),
			msg.Payload(),
		)

		return
	}

	glog.Infof("Received from topic: %v\n>>\t%s", msg.Topic(), msg.Payload())
	watcher.OnlineMessage <- msg.Payload()
}
