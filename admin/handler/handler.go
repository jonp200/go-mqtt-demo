package handler

import (
	"encoding/json"
	"fmt"
	"net/http"
	"os"

	"github.com/gorilla/websocket"
	"github.com/labstack/echo/v4"
	glog "github.com/labstack/gommon/log"
	"go-mqtt-demo/client"
)

type Handler struct {
	cfgClient    *client.Mqtt
	sensorClient *client.WebSocket
	locId        string
}

func New(ca, cfgClientId, sensorClientId string) *Handler {
	locId := os.Getenv("LOCATION_ID")
	cfgClient := client.NewMqtt(ca, cfgClientId)

	if token := cfgClient.Connect(); token.Wait() && token.Error() != nil {
		glog.Fatal(token.Error())
	}

	topic := fmt.Sprintf("location/%v/kiosk/+/sensor/#", locId)
	sensorClient := client.NewWebSocket(ca, sensorClientId, topic)

	if token := sensorClient.Connect(); token.Wait() && token.Error() != nil {
		glog.Fatal(token.Error())
	}

	return &Handler{
		cfgClient:    cfgClient,
		sensorClient: sensorClient,
		locId:        locId,
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

// WsSensor handles the subscription to all kiosks' sensors in the location.
func (h *Handler) WsSensor(c echo.Context) error {
	upgrader := websocket.Upgrader{}
	conn, err := upgrader.Upgrade(c.Response(), c.Request(), nil)

	if err != nil {
		glog.Errorf("WebSocket upgrade failed: %v", err)

		return err
	}

	defer conn.Close()

	h.sensorClient.WsEvent <- client.WebSocketEvent{Conn: conn, Action: "add"}

	for {
		if _, _, err = conn.ReadMessage(); err != nil {
			h.sensorClient.WsEvent <- client.WebSocketEvent{Conn: conn, Action: "remove"}

			break
		}
	}

	return nil
}
