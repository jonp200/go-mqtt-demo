package client

import (
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"net/http"
	"net/url"
	"os"
	"sort"
	"sync"
	"sync/atomic"
	"time"

	mqtt "github.com/eclipse/paho.mqtt.golang"
	"github.com/gorilla/websocket"
	glog "github.com/labstack/gommon/log"
)

type Mqtt struct {
	mqtt.Client
}

func NewMqtt(caName, clientId string) (*Mqtt, error) {
	opts := mqtt.NewClientOptions().
		AddBroker(fmt.Sprintf("mqtts://%v:%v", os.Getenv("BROKER_ADDRESS"), os.Getenv("BROKER_PORT"))).
		SetDefaultPublishHandler(defaultPublishHandler)

	if err := setTLSConfig(opts, caName); err != nil {
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

type WebSocketEvent struct {
	Id     int64
	Conn   *websocket.Conn
	Action string
}

type SseEvent struct {
	Writer http.ResponseWriter
	Done   chan bool
}

type SseMessage struct {
	Data []byte
}

type OfflineMessage struct {
	Id      int64
	Payload []byte
}

type ConnEventWatcher struct {
	WsEvent       chan WebSocketEvent
	WsConnections sync.Map
	OnlineMessage chan []byte

	SseEvent       chan SseEvent
	SseMessages    sync.Map
	OfflineMessage chan OfflineMessage

	done chan struct{}
}

func NewConnEventWatcher() *ConnEventWatcher {
	const eventsBuffer = 10

	const messageBuffer = 256

	w := &ConnEventWatcher{
		WsConnections: sync.Map{},
		WsEvent:       make(chan WebSocketEvent, eventsBuffer),
		OnlineMessage: make(chan []byte, messageBuffer),

		SseMessages:    sync.Map{},
		SseEvent:       make(chan SseEvent, eventsBuffer),
		OfflineMessage: make(chan OfflineMessage, messageBuffer),

		done: make(chan struct{}),
	}

	go w.run()

	return w
}

func (w *ConnEventWatcher) Stop() {
	close(w.done)
}

func (w *ConnEventWatcher) run() {
	for {
		select {
		case <-w.done:
			return
		case event := <-w.WsEvent:
			switch event.Action {
			case "add":
				w.WsConnections.Store(event.Id, event.Conn)
				glog.Info("WebSocket connection added")
			case "remove":
				w.WsConnections.Delete(event.Id)
				glog.Info("WebSocket connection removed")
			}
		case msg := <-w.OnlineMessage:
			w.relayOnlineMessages(msg)
		case event := <-w.SseEvent:
			w.replayOfflineMessages(event)
		case msg := <-w.OfflineMessage:
			w.SseMessages.Store(msg.Id, SseMessage{Data: msg.Payload})
		}
	}
}

func (w *ConnEventWatcher) relayOnlineMessages(msg []byte) {
	// WebSocket connections order does not matter, so we can iterate over the map without sorting the connections
	w.WsConnections.Range(
		func(k, v any) bool {
			conn, ok := v.(*websocket.Conn)
			if !ok {
				glog.Errorf("Invalid data: %v", v)

				w.WsConnections.Delete(k)
				glog.Error("WebSocket connection removed")

				return false
			}

			if err := conn.WriteMessage(websocket.TextMessage, msg); err != nil {
				glog.Errorf("Failed to send message: %v", err)

				_ = conn.Close()

				w.WsConnections.Delete(k)
				glog.Error("WebSocket connection removed")

				return false
			}

			return true
		},
	)
}

func (w *ConnEventWatcher) replayOfflineMessages(event SseEvent) {
	iterate := func(k any, r bool) bool {
		w.SseMessages.Delete(k)

		return r
	}

	// Offline messages should be written in order, so we'll sort the messages by their ID
	var keys []int64

	entries := make(map[int64]SseMessage)

	w.SseMessages.Range(
		func(k, v any) bool {
			msg, ok := v.(SseMessage)
			if !ok {
				glog.Errorf("Invalid data: %v", v)

				return iterate(k, false)
			}

			key, ok := k.(int64)
			if !ok {
				glog.Errorf("Invalid key value: %v", k)

				return iterate(k, false)
			}

			keys = append(keys, key)
			entries[key] = msg

			return iterate(k, true)
		},
	)

	sort.Slice(
		keys, func(i, j int) bool {
			return keys[i] < keys[j]
		},
	)

	// Use the key slice to process in order
	for _, k := range keys {
		if _, err := fmt.Fprintf(event.Writer, "data: %s\n\n", entries[k].Data); err != nil {
			glog.Errorf("Failed to write message: %v", err)

			break
		}
	}

	event.Done <- true
}

type WebSocket struct {
	mqtt.Client
	*ConnEventWatcher
}

// WsQos is the QoS when subscribing. Preferred to use QoS level 1 when subscribing.
const WsQos = 1

func NewWebSocket(caName, clientId, topic string) (*WebSocket, error) {
	opts := mqtt.NewClientOptions().
		AddBroker(fmt.Sprintf("wss://%v:%v/mqtt", os.Getenv("BROKER_ADDRESS"), os.Getenv("BROKER_WS_PORT")))

	if err := setTLSConfig(opts, caName); err != nil {
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

func setTLSConfig(opts *mqtt.ClientOptions, caName string) error {
	certpool := x509.NewCertPool()

	ca, err := os.ReadFile(caName)
	if err != nil {
		return err
	}

	certpool.AppendCertsFromPEM(ca)
	opts.SetTLSConfig(
		&tls.Config{
			RootCAs: certpool,
		},
	)

	return nil
}

func setAuth(clientId string, opts *mqtt.ClientOptions) {
	// The resulting client ID must be unique, otherwise the broker will reject the connection
	opts.SetClientID(clientId + "_" + os.Getenv("CLIENT_ID_SUFFIX"))
	opts.SetUsername(os.Getenv("MQTT_USERNAME"))
	opts.SetPassword(os.Getenv("MQTT_PASSWORD"))
}

func setCleanSession(opts *mqtt.ClientOptions) {
	opts.CleanSession = os.Getenv("MQTT_CLEAN_SESSION") == "1"
}

func setReconnect(opts *mqtt.ClientOptions) {
	opts.SetAutoReconnect(true)

	maxReconnectInterval, _ := time.ParseDuration(os.Getenv("MQTT_MAX_RECONNECT_INTERVAL"))
	opts.SetMaxReconnectInterval(maxReconnectInterval)
}

func defaultPublishHandler(_ mqtt.Client, msg mqtt.Message) {
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
