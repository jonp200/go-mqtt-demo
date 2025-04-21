package client

import (
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"net/url"
	"os"

	mqtt "github.com/eclipse/paho.mqtt.golang"
	"github.com/gorilla/websocket"
	glog "github.com/labstack/gommon/log"
)

func setTLSConfig(opts *mqtt.ClientOptions, caName string) {
	certpool := x509.NewCertPool()
	ca, err := os.ReadFile(caName)
	if err != nil {
		glog.Fatal(err.Error())
	}
	certpool.AppendCertsFromPEM(ca)
	opts.SetTLSConfig(
		&tls.Config{
			RootCAs: certpool,
		},
	)
}

func setAuth(clientId string, opts *mqtt.ClientOptions) {
	opts.SetClientID(clientId) // client ID must be unique
	opts.SetUsername(os.Getenv("MQTT_USERNAME"))
	opts.SetPassword(os.Getenv("MQTT_PASSWORD"))
}

func setCleanSession(opts *mqtt.ClientOptions) {
	opts.CleanSession = os.Getenv("MQTT_CLEAN_SESSION") == "1"
}

func publishHandler(_ mqtt.Client, msg mqtt.Message) {
	glog.Infof("Received from topic: %v\n>>\t%s", msg.Topic(), msg.Payload())
}

func onConnectAttempt(broker *url.URL, tlsCfg *tls.Config) *tls.Config {
	glog.Infof("Connecting to broker %s...", broker.Host)
	return tlsCfg
}

func onReconnecting(_ mqtt.Client, opts *mqtt.ClientOptions) {
	glog.Infof("Reconnecting to broker %s...", opts.Servers)
}

func onConnect(_ mqtt.Client) {
	glog.Info("Connected to broker")
}

func onConnectionLost(_ mqtt.Client, err error) {
	glog.Infof("Connection to broker lost: %v", err)
}

type Mqtt struct {
	mqtt.Client
}

func NewMqtt(caName, clientId string) *Mqtt {
	opts := mqtt.NewClientOptions().
		AddBroker(fmt.Sprintf("mqtts://%v:%v", os.Getenv("BROKER_ADDRESS"), os.Getenv("BROKER_PORT"))).
		SetDefaultPublishHandler(publishHandler)

	setTLSConfig(opts, caName)
	setAuth(clientId, opts)
	setCleanSession(opts)

	opts.OnConnectAttempt = onConnectAttempt
	opts.OnReconnecting = onReconnecting
	opts.OnConnect = onConnect
	opts.OnConnectionLost = onConnectionLost

	return &Mqtt{mqtt.NewClient(opts)}
}

type WebSocketEvent struct {
	Conn   *websocket.Conn
	Action string // "add" or "remove"
}

type WebSocketEventWatcher struct {
	Connections map[*websocket.Conn]bool
	Event       chan WebSocketEvent
	Message     chan []byte
}

func (w *WebSocketEventWatcher) run() {
	for {
		select {
		case event := <-w.Event:
			switch event.Action {
			case "add":
				glog.Info("WebSocket connection added")
				w.Connections[event.Conn] = true
			case "remove":
				glog.Info("WebSocket connection removed")
				delete(w.Connections, event.Conn)
			}
		case msg := <-w.Message:
			for conn := range w.Connections {
				if err := conn.WriteMessage(websocket.TextMessage, msg); err != nil {
					glog.Errorf("Failed to send message: %v", err)
					_ = conn.Close()
					delete(w.Connections, conn)
					glog.Error("WebSocket connection removed")
				}
			}
		}
	}
}

func NewWebSocketEventWatcher() *WebSocketEventWatcher {
	w := &WebSocketEventWatcher{
		Connections: make(map[*websocket.Conn]bool),
		Event:       make(chan WebSocketEvent),
		Message:     make(chan []byte),
	}

	go w.run()
	return w
}

type WebSocket struct {
	mqtt.Client
	*WebSocketEventWatcher
}

func NewWebSocket(caName, clientId, topic string) *WebSocket {
	opts := mqtt.NewClientOptions().
		AddBroker(fmt.Sprintf("wss://%v:%v/mqtt", os.Getenv("BROKER_ADDRESS"), os.Getenv("BROKER_WS_PORT")))

	setTLSConfig(opts, caName)
	setAuth(clientId, opts)
	setCleanSession(opts)

	opts.OnConnectAttempt = onConnectAttempt
	opts.OnReconnecting = onReconnecting

	watcher := NewWebSocketEventWatcher()
	opts.OnConnect = func(client mqtt.Client) {
		glog.Infof("Connected to broker over WebSocket")

		const qos = 1 // preferred to use QOS 1 when subscribing
		token := client.Subscribe(
			topic, qos, func(_ mqtt.Client, msg mqtt.Message) {
				glog.Infof("Received from topic: %v\n>>\t%s", msg.Topic(), msg.Payload())
				watcher.Message <- msg.Payload()
			},
		)
		if err := token.Error(); token.Wait() && err != nil {
			glog.Errorf("Subscribe error: %v", err)
		}
	}

	opts.OnConnectionLost = onConnectionLost

	return &WebSocket{
		Client:                mqtt.NewClient(opts),
		WebSocketEventWatcher: watcher,
	}
}
