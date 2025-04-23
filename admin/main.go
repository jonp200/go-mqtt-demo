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

	e := echo.New()

	e.Renderer = &tmpl.Renderer{Template: template.Must(template.ParseGlob("public/*.html"))}

	e.File("/favicon.ico", "images/favicon.ico")
	e.GET(
		"/publisher", func(c echo.Context) error {
			data := map[string]interface{}{
				"Port":  port,
				"LocId": locId,
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

	// Client for publishing to kiosk config in the location
	const cfgClientId = "pub_cfg_client"
	cfgClient := client.NewMqtt(ca, cfgClientId)
	if token := cfgClient.Connect(); token.Wait() && token.Error() != nil {
		glog.Fatal(token.Error())
	}

	// Client for subscribing to all kiosks' sensors in the location
	const sensorClientId = "sub_sensor_client"
	topic := fmt.Sprintf("location/%v/kiosk/+/sensor/#", locId)
	sensorClient := client.NewWebSocket(ca, sensorClientId, topic)
	if token := sensorClient.Connect(); token.Wait() && token.Error() != nil {
		glog.Fatal(token.Error())
	}

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
			topic := fmt.Sprintf("location/%v/kiosk/config", locId)
			if token := cfgClient.Publish(topic, qos, false, data); token.Wait() && token.Error() != nil {
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
		"/ws/sensor", func(c echo.Context) error {
			upgrader := websocket.Upgrader{}
			conn, err := upgrader.Upgrade(c.Response(), c.Request(), nil)
			if err != nil {
				glog.Errorf("WebSocket upgrade failed: %v", err)
				return err
			}
			defer conn.Close()

			sensorClient.WsEvent <- client.WebSocketEvent{Conn: conn, Action: "add"}

			for {
				if _, _, err = conn.ReadMessage(); err != nil {
					sensorClient.WsEvent <- client.WebSocketEvent{Conn: conn, Action: "remove"}
					break
				}
			}

			return nil
		},
	)

	e.Logger.Fatal(e.Start(":" + port))
}
