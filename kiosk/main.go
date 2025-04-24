package main

import (
	"html/template"
	"net/http"
	"os"
	"os/signal"

	"github.com/joho/godotenv"
	"github.com/labstack/echo/v4"
	glog "github.com/labstack/gommon/log"
	tmpl "go-mqtt-demo/html/template"
	"go-mqtt-demo/kiosk/handler"
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

	frontend(e, port, locId, kioskId)

	const ca = "emqxsl-ca.crt"

	h := handler.New(ca, "pub_sensor_client", "sub_cfg_client")

	e.POST("/sensor1", h.Sensor1)
	e.GET("/offline-message/config", h.SseConfig)
	e.GET("/ws/config", h.WsConfig)

	c := make(chan os.Signal, 1)
	signal.Notify(c, os.Interrupt)

	go func() {
		<-c

		const delay = 250

		h.Disconnect(delay)
		os.Exit(0)
	}()

	e.Logger.Fatal(e.Start(":" + port))
}

func frontend(e *echo.Echo, port, locId, kioskId string) {
	e.Renderer = tmpl.Renderer{Template: template.Must(template.ParseGlob("public/*.html"))}

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
}
