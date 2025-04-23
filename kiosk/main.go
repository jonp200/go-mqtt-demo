package main

import (
	"encoding/json"
	"fmt"
	"html/template"
	"net/http"
	"os"

	"github.com/gorilla/websocket"
	"github.com/joho/godotenv"
	"github.com/labstack/echo/v4"
	glog "github.com/labstack/gommon/log"
	"go-mqtt-demo/client"
	tmpl "go-mqtt-demo/html/template"
	"go-mqtt-demo/logger"
)

func main() {
	if err := godotenv.Load(); err != nil {
		glog.Fatal("Error loading .env file")
	}
	logger.Init()

	port := os.Getenv("SERVICE_PORT")
	locId := os.Getenv("LOCATION_ID")
	kioskId := os.Getenv("KIOSK_ID")

	e := echo.New()

	e.Renderer = &tmpl.Renderer{Template: template.Must(template.ParseGlob("public/*.html"))}

	e.File("/favicon.ico", "images/favicon.ico")
	e.GET(
		"/publisher", func(c echo.Context) error {
			data := map[string]interface{}{
				"Port":    port,
				"LocId":   locId,
				"KioskId": kioskId,
			}
			return c.Render(http.StatusOK, "publisher.html", data)
		},
	)
	e.GET(
		"/subscriber", func(c echo.Context) error {
			data := map[string]interface{}{
				"Port": port,
			}
			return c.Render(http.StatusOK, "subscriber.html", data)
		},
	)

	const ca = "emqxsl-ca.crt"

	// Client for publishing to the kiosk sensor in the location
	const sensorClientId = "pub_sensor_client"
	sensorClient := client.NewMqtt(ca, sensorClientId)
	if token := sensorClient.Connect(); token.Wait() && token.Error() != nil {
		glog.Fatal(token.Error())
	}

	// Client for subscribing to kiosk config in the location
	const cfgClientId = "sub_cfg_client"
	cfgTopic := fmt.Sprintf("location/%v/kiosk/config", locId)
	cfgClient := client.NewWebSocket(ca, cfgClientId, cfgTopic)
	if token := cfgClient.Connect(); token.Wait() && token.Error() != nil {
		glog.Fatal(token.Error())
	}

	e.POST(
		"/sensor1", func(c echo.Context) error {
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
			topic := fmt.Sprintf("location/%v/kiosk/%v/sensor/1", locId, kioskId)
			if token := sensorClient.Publish(topic, qos, false, data); token.Wait() && token.Error() != nil {
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

	e.Logger.Fatal(e.Start(":" + port))
}
