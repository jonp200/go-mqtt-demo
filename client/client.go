package client

import (
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"net/http"
	"net/url"
	"os"

	mqtt "github.com/eclipse/paho.mqtt.golang"
	"github.com/google/uuid"
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
	opts.SetClientID(clientId + "_" + os.Getenv("CLIENT_ID_SUFFIX")) // client ID must be unique
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
	Action string
}

type SseEvent struct {
	Writer http.ResponseWriter
}

type SseMessage struct {
	Data []byte
}

type OfflineMessage struct {
	Id      uuid.UUID
	Payload []byte
}

type ConnEventWatcher struct {
	WsEvent       chan WebSocketEvent
	WsConn        map[*websocket.Conn]bool
	OnlineMessage chan []byte

	SseMessages    map[uuid.UUID]SseMessage
	OfflineMessage chan OfflineMessage
}

func (w *ConnEventWatcher) run() {
	for {
		select {
		case event := <-w.WsEvent:
			switch event.Action {
			case "add":
				w.WsConn[event.Conn] = true
				glog.Info("WebSocket connection added")
			case "remove":
				delete(w.WsConn, event.Conn)
				glog.Info("WebSocket connection removed")
			}
		case msg := <-w.OnlineMessage:
			for conn := range w.WsConn {
				if err := conn.WriteMessage(websocket.TextMessage, msg); err != nil {
					glog.Errorf("Failed to send message: %v", err)
					_ = conn.Close()
					delete(w.WsConn, conn)
					glog.Error("WebSocket connection removed")
				}
			}
		case msg := <-w.OfflineMessage:
			w.SseMessages[msg.Id] = SseMessage{msg.Payload}
		}
	}
}

func NewConnEventWatcher() *ConnEventWatcher {
	w := &ConnEventWatcher{
		WsConn:         make(map[*websocket.Conn]bool),
		WsEvent:        make(chan WebSocketEvent),
		SseMessages:    make(map[uuid.UUID]SseMessage),
		OnlineMessage:  make(chan []byte),
		OfflineMessage: make(chan OfflineMessage),
	}

	go w.run()
	return w
}

type WebSocket struct {
	mqtt.Client
	*ConnEventWatcher
}

func NewWebSocket(caName, clientId, topic string) *WebSocket {
	opts := mqtt.NewClientOptions().
		AddBroker(fmt.Sprintf("wss://%v:%v/mqtt", os.Getenv("BROKER_ADDRESS"), os.Getenv("BROKER_WS_PORT")))

	setTLSConfig(opts, caName)
	setAuth(clientId, opts)
	setCleanSession(opts)

	opts.OnConnectAttempt = onConnectAttempt
	opts.OnReconnecting = onReconnecting

	watcher := NewConnEventWatcher()
	opts.OnConnect = func(client mqtt.Client) {
		const qos = 1 // preferred to use QOS 1 when subscribing
		init := true
		token := client.Subscribe(
			topic, qos, func(_ mqtt.Client, msg mqtt.Message) {
				// Send messages found upon init to the offline message chan
				if init {
					watcher.OfflineMessage <- OfflineMessage{
						Id:      uuid.New(), // MessageID() is always 0. Maybe there's a config necessary?
						Payload: msg.Payload(),
					}
					glog.Infof("Message (while offline) from topic: %v\n>>\t%s", msg.Topic(), msg.Payload())
					return
				}

				glog.Infof("Received from topic: %v\n>>\t%s", msg.Topic(), msg.Payload())
				watcher.OnlineMessage <- msg.Payload()
			},
		)

		if err := token.Error(); token.Wait() && err != nil {
			glog.Errorf("Subscribe error: %v", err)
		}

		glog.Infof("Connected to broker over WebSocket")
		init = false
	}

	opts.OnConnectionLost = onConnectionLost

	return &WebSocket{
		Client:           mqtt.NewClient(opts),
		ConnEventWatcher: watcher,
	}
}
