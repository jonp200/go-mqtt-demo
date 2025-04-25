package handler

import (
	"encoding/json"
	"net/http"
	"sync"
	"time"

	"github.com/gorilla/websocket"
	"github.com/labstack/echo/v4"
	glog "github.com/labstack/gommon/log"
	"go-mqtt-demo/client"
)

type Handler struct {
	mqtt *client.Mqtt
	ws   *client.WebSocket
}

func New(ca, pubClientId, subClientId, subTopic string) (*Handler, error) {
	pub, err := client.NewMqtt(ca, pubClientId)
	if err != nil {
		return nil, err
	}

	sub, err := client.NewWebSocket(ca, subClientId, subTopic)
	if err != nil {
		return nil, err
	}

	return &Handler{mqtt: pub, ws: sub}, nil
}

func (h *Handler) Connect() error {
	if token := h.mqtt.Connect(); token.Wait() && token.Error() != nil {
		return token.Error()
	}

	if token := h.ws.Connect(); token.Wait() && token.Error() != nil {
		return token.Error()
	}

	return nil
}

func (h *Handler) Disconnect() {
	const tasks = 2

	var wg sync.WaitGroup

	wg.Add(tasks)

	go func() {
		defer wg.Done()
		h.mqtt.Disconnect()
	}()

	go func() {
		defer wg.Done()
		h.ws.Disconnect()
	}()

	wg.Wait()
}

func (h *Handler) Publish(c echo.Context) error {
	var p struct {
		Topic string `json:"topic"`
		Data  any    `json:"data"`
	}

	if err := c.Bind(&p); err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, err)
	}

	data, err := json.Marshal(p.Data)
	if err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, err)
	}

	const qos = 0
	if token := h.mqtt.Publish(p.Topic, qos, false, data); token.Wait() && token.Error() != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, err)
	}

	return c.JSON(
		http.StatusOK, echo.Map{
			"message": "ok",
		},
	)
}

func (h *Handler) SubscribeSse(c echo.Context) error {
	c.Response().Header().Set("Content-Type", "text/event-stream")
	c.Response().Header().Set("Cache-Control", "no-cache")
	c.Response().Header().Set("Connection", "keep-alive")

	f, ok := c.Response().Writer.(http.Flusher)
	if !ok {
		glog.Errorf("Failed to flush response stream: %v", c.Response().Writer)

		return nil
	}

	done := make(chan bool)
	h.ws.SseEvent <- client.SseEvent{Writer: c.Response().Writer, Done: done}

	<-done
	f.Flush()

	return nil
}

func (h *Handler) SubscribeWs(c echo.Context) error {
	upgrader := websocket.Upgrader{}
	conn, err := upgrader.Upgrade(c.Response(), c.Request(), nil)

	if err != nil {
		glog.Errorf("WebSocket upgrade failed: %v", err)

		return err
	}

	defer conn.Close()

	eventId := time.Now().UnixNano()
	h.ws.WsEvent <- client.WebSocketEvent{Id: eventId, Conn: conn, Action: "add"}

	for {
		if _, _, err = conn.ReadMessage(); err != nil {
			h.ws.WsEvent <- client.WebSocketEvent{Id: eventId, Conn: conn, Action: "remove"}

			break
		}
	}

	return nil
}
