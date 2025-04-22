package main

import (
	"encoding/json"
	"fmt"
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

	// Client for publishing to the kiosk sensor in the location
	sensorClient := client.NewMqtt("emqxsl-ca.crt", "pub_loc1_kiosk1_sensor")
	if token := sensorClient.Connect(); token.Wait() && token.Error() != nil {
		glog.Fatal(token.Error())
	}

	// Client for subscribing to kiosk config in the location
	cfgClient := client.NewWebSocket("emqxsl-ca.crt", "sub_loc1_kiosk_cfg", "location/1/kiosk/config")
	if token := cfgClient.Connect(); token.Wait() && token.Error() != nil {
		glog.Fatal(token.Error())
	}

	e.POST(
		"/sensor", func(c echo.Context) error {
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
			if token := sensorClient.Publish(
				"location/1/kiosk/1/sensor/1", qos, false, data,
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

	e.GET(
		"/offline-message/config", func(c echo.Context) error {
			c.Response().Header().Set("Content-Type", "text/event-stream")
			c.Response().Header().Set("Cache-Control", "no-cache")
			c.Response().Header().Set("Connection", "keep-alive")

			f, ok := c.Response().Writer.(http.Flusher)
			if !ok {
				glog.Errorf("Failed to flush response stream: %v", c.Response().Writer)
				return nil
			}

			for k, v := range cfgClient.SseMessages {
				str := fmt.Sprintf("data: %s\n\n", v.Data)
				if _, err := c.Response().Writer.Write([]byte(str)); err != nil {
					glog.Errorf("Failed to write message: %v", err)
				}

				f.Flush()
				delete(cfgClient.SseMessages, k)
			}

			return nil
		},
	)

	e.GET(
		"/ws/config", func(c echo.Context) error {
			upgrader := websocket.Upgrader{}
			conn, err := upgrader.Upgrade(c.Response(), c.Request(), nil)
			if err != nil {
				glog.Errorf("WebSocket upgrade failed: %v", err)
				return err
			}
			defer conn.Close()

			cfgClient.WsEvent <- client.WebSocketEvent{Conn: conn, Action: "add"}

			for {
				if _, _, err = conn.ReadMessage(); err != nil {
					cfgClient.WsEvent <- client.WebSocketEvent{Conn: conn, Action: "remove"}
					break
				}
			}

			return nil
		},
	)

	e.Logger.Fatal(e.Start(":9191"))
}
