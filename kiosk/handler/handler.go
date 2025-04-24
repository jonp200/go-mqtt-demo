package handler

import (
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"sync"
	"time"

	"github.com/gorilla/websocket"
	"github.com/labstack/echo/v4"
	glog "github.com/labstack/gommon/log"
	"go-mqtt-demo/client"
)

type Handler struct {
	sensorClient *client.Mqtt
	cfgClient    *client.WebSocket
	locId        string
	kioskId      string
}

func New(ca, sensorClientId, cfgClientId string) *Handler {
	locId := os.Getenv("LOCATION_ID")
	kioskId := os.Getenv("KIOSK_ID")

	sensorClient := client.NewMqtt(ca, sensorClientId)

	if token := sensorClient.Connect(); token.Wait() && token.Error() != nil {
		glog.Fatal(token.Error())
	}

	cfgTopic := fmt.Sprintf("location/%v/kiosk/config", locId)
	cfgClient := client.NewWebSocket(ca, cfgClientId, cfgTopic)

	if token := cfgClient.Connect(); token.Wait() && token.Error() != nil {
		glog.Fatal(token.Error())
	}

	return &Handler{
		sensorClient: sensorClient,
		cfgClient:    cfgClient,
		locId:        locId,
		kioskId:      kioskId,
	}
}

func (h *Handler) Disconnect(delay uint) {
	glog.Infof("Disconnecting clients...")

	const tasks = 2

	var wg sync.WaitGroup

	wg.Add(tasks)

	go func() {
		defer wg.Done()
		h.sensorClient.Disconnect(delay)
	}()

	go func() {
		defer wg.Done()
		h.cfgClient.Disconnect(delay)
	}()

	wg.Wait()
	glog.Infof("Clients disconnected")
}

// Sensor1 handles the publishing of the kiosk sensor to the location.
func (h *Handler) Sensor1(c echo.Context) error {
	var p struct {
		Data any `json:"data"`
	}

	if err := c.Bind(&p); err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, err)
	}

	data, err := json.Marshal(p.Data)
	if err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, err)
	}

	const qos = 0

	topic := fmt.Sprintf("location/%v/kiosk/%v/sensor/1", h.locId, h.kioskId)

	if token := h.sensorClient.Publish(topic, qos, false, data); token.Wait() && token.Error() != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, err)
	}

	return c.JSON(
		http.StatusOK, echo.Map{
			"message": "ok",
		},
	)
}

// SseConfig handles the sending of config messages received while the service was offline.
func (h *Handler) SseConfig(c echo.Context) error {
	c.Response().Header().Set("Content-Type", "text/event-stream")
	c.Response().Header().Set("Cache-Control", "no-cache")
	c.Response().Header().Set("Connection", "keep-alive")

	f, ok := c.Response().Writer.(http.Flusher)
	if !ok {
		glog.Errorf("Failed to flush response stream: %v", c.Response().Writer)

		return nil
	}

	done := make(chan bool)
	h.cfgClient.SseEvent <- client.SseEvent{Writer: c.Response().Writer, Done: done}

	<-done
	f.Flush()

	return nil
}

// WsConfig handles the subscription to kiosk config in the location.
func (h *Handler) WsConfig(c echo.Context) error {
	upgrader := websocket.Upgrader{}
	conn, err := upgrader.Upgrade(c.Response(), c.Request(), nil)

	if err != nil {
		glog.Errorf("WebSocket upgrade failed: %v", err)

		return err
	}

	defer conn.Close()

	eventId := time.Now().UnixNano()
	h.cfgClient.WsEvent <- client.WebSocketEvent{Id: eventId, Conn: conn, Action: "add"}

	for {
		if _, _, err = conn.ReadMessage(); err != nil {
			h.cfgClient.WsEvent <- client.WebSocketEvent{Id: eventId, Conn: conn, Action: "remove"}

			break
		}
	}

	return nil
}
