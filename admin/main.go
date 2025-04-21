package main

import (
	"encoding/json"
	"net/http"

	"github.com/gorilla/websocket"
	"github.com/joho/godotenv"
	"github.com/labstack/echo/v4"
	glog "github.com/labstack/gommon/log"
	"go-mqtt-demo/client"
	"go-mqtt-demo/logger"
)

func main() {
	if err := godotenv.Load(); err != nil {
		glog.Fatal("Error loading .env file")
	}
	logger.Init()

	e := echo.New()

	e.File("/favicon.ico", "images/favicon.ico")
	e.File("/publisher", "public/publisher.html")
	e.File("/subscriber", "public/subscriber.html")

	// Client for subscribing to all kiosks' sensors in the location
	sensorClient := client.NewWebSocket("emqxsl-ca.crt", "loc1_kiosk_sensor", "location/1/kiosk/+/sensor/#")
	if token := sensorClient.Connect(); token.Wait() && token.Error() != nil {
		glog.Fatal(token.Error())
	}

	// Client for publishing to kiosk config in the location
	cfgClient := client.NewMqtt("emqxsl-ca.crt", "loc1_kiosk_cfg")
	if token := cfgClient.Connect(); token.Wait() && token.Error() != nil {
		glog.Fatal(token.Error())
	}

	e.GET(
		"/ws/sensor", func(c echo.Context) error {
			upgrader := websocket.Upgrader{}
			conn, err := upgrader.Upgrade(c.Response(), c.Request(), nil)
			if err != nil {
				glog.Errorf("WebSocket upgrade failed: %v", err)
				return err
			}
			defer conn.Close()

			sensorClient.Event <- client.WebSocketEvent{Conn: conn, Action: "add"}

			for {
				if _, _, err = conn.ReadMessage(); err != nil {
					sensorClient.Event <- client.WebSocketEvent{Conn: conn, Action: "remove"}
					break
				}
			}

			return nil
		},
	)

	e.POST(
		"/config", func(c echo.Context) error {
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
			if token := cfgClient.Publish(
				"location/1/kiosk/config", qos, false, data,
			); token.Wait() && token.Error() != nil {
				return echo.NewHTTPError(http.StatusInternalServerError, err)
			}

			return c.JSON(
				http.StatusOK, echo.Map{
					"message": "ok",
				},
			)
		},
	)

	e.Logger.Fatal(e.Start(":9090"))
}
