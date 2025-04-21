package main

import (
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

	e.File("/subscriber", "public/subscriber.html")
	e.File("/favicon.ico", "images/favicon.ico")

	cfgClient := client.NewWebSocket("emqxsl-ca.crt", "location/1/kiosk/config")
	if token := cfgClient.Connect(); token.Wait() && token.Error() != nil {
		glog.Fatal(token.Error())
	}

	e.GET(
		"/ws/config", func(c echo.Context) error {
			upgrader := websocket.Upgrader{}
			conn, err := upgrader.Upgrade(c.Response(), c.Request(), nil)
			if err != nil {
				glog.Errorf("WebSocket upgrade failed: %v", err)
				return err
			}
			defer conn.Close()

			cfgClient.Event <- client.WebSocketEvent{Conn: conn, Action: "add"}

			for {
				if _, _, err = conn.ReadMessage(); err != nil {
					cfgClient.Event <- client.WebSocketEvent{Conn: conn, Action: "remove"}
					break
				}
			}

			return nil
		},
	)

	e.Logger.Fatal(e.Start(":9191"))
}
