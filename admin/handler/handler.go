package handler

import (
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"time"

	"github.com/gorilla/websocket"
	"github.com/labstack/echo/v4"
	glog "github.com/labstack/gommon/log"
	"go-mqtt-demo/client"
)

type Handler struct {
	cfgClient     *client.Mqtt
	sensorsClient *client.WebSocket
	locId         string
}

func New(ca, cfgClientId, sensorClientId string) *Handler {
	locId := os.Getenv("LOCATION_ID")
	cfgClient := client.NewMqtt(ca, cfgClientId)

	if token := cfgClient.Connect(); token.Wait() && token.Error() != nil {
		glog.Fatal(token.Error())
	}

	topic := fmt.Sprintf("location/%v/kiosk/+/sensor/#", locId)
	sensorsClient := client.NewWebSocket(ca, sensorClientId, topic)

	if token := sensorsClient.Connect(); token.Wait() && token.Error() != nil {
		glog.Fatal(token.Error())
	}

	return &Handler{
		cfgClient:     cfgClient,
		sensorsClient: sensorsClient,
		locId:         locId,
	}
}

// Config handles the publishing of kiosk config to the location.
func (h *Handler) Config(c echo.Context) error {
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

	topic := fmt.Sprintf("location/%v/kiosk/config", h.locId)

	if token := h.cfgClient.Publish(topic, qos, false, data); token.Wait() && token.Error() != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, err)
	}

	return c.JSON(
		http.StatusOK, echo.Map{
			"message": "ok",
		},
	)
}

// SseSensors handles the sending of sensor messages received while the service was offline.
func (h *Handler) SseSensors(c echo.Context) error {
	c.Response().Header().Set("Content-Type", "text/event-stream")
	c.Response().Header().Set("Cache-Control", "no-cache")
	c.Response().Header().Set("Connection", "keep-alive")

	f, ok := c.Response().Writer.(http.Flusher)
	if !ok {
		glog.Errorf("Failed to flush response stream: %v", c.Response().Writer)

		return nil
	}

	done := make(chan bool)
	h.sensorsClient.SseEvent <- client.SseEvent{Writer: c.Response().Writer, Done: done}

	<-done
	f.Flush()

	return nil
}

// WsSensors handles the subscription to all kiosks' sensors in the location.
func (h *Handler) WsSensors(c echo.Context) error {
	upgrader := websocket.Upgrader{}
	conn, err := upgrader.Upgrade(c.Response(), c.Request(), nil)

	if err != nil {
		glog.Errorf("WebSocket upgrade failed: %v", err)

		return err
	}

	defer conn.Close()

	eventId := time.Now().UnixNano()
	h.sensorsClient.WsEvent <- client.WebSocketEvent{Id: eventId, Conn: conn, Action: "add"}

	for {
		if _, _, err = conn.ReadMessage(); err != nil {
			h.sensorsClient.WsEvent <- client.WebSocketEvent{Id: eventId, Conn: conn, Action: "remove"}

			break
		}
	}

	return nil
}
