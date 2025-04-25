package client

import (
	"fmt"
	"net/http"
	"sort"
	"sync"

	"github.com/gorilla/websocket"
	glog "github.com/labstack/gommon/log"
)

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
