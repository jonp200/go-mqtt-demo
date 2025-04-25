package main

import (
	"fmt"
	"html/template"
	"net/http"
	"os"
	"os/signal"

	"github.com/joho/godotenv"
	"github.com/labstack/echo/v4"
	glog "github.com/labstack/gommon/log"
	"go-mqtt-demo/config"
	"go-mqtt-demo/handler"
	tmpl "go-mqtt-demo/html/template"
	"go-mqtt-demo/logger"
)

func main() {
	if err := godotenv.Load(); err != nil {
		glog.Fatal("Error loading .env file")
	}

	logger.Init()

	e := echo.New()

	cfg, err := config.New()
	if err != nil {
		glog.Fatal(err)
	}

	frontend(e, cfg)

	const ca = "emqxsl-ca.crt"

	subTopic := fmt.Sprintf("location/%v/kiosk/config", cfg.LocationId)

	h, err := handler.New(ca, "pub_sensor_client", "sub_cfg_client", subTopic)
	if err != nil {
		glog.Fatal(err)
	}

	if err := h.Connect(); err != nil {
		glog.Fatal(err)
	}

	e.POST("/sensor1", h.Publish)

	e.GET("/sse/config", h.SubscribeSse)
	e.GET("/ws/config", h.SubscribeWs)

	handleShutdown(h)

	e.Logger.Fatal(e.Start(":" + cfg.ServicePort))
}

func frontend(e *echo.Echo, cfg *config.Config) {
	e.Renderer = tmpl.Renderer{Template: template.Must(template.ParseGlob("web/*.html"))}

	e.File("/favicon.ico", "images/favicon.ico")
	e.GET(
		"/pub", func(c echo.Context) error {
			data := map[string]interface{}{
				"Port":    cfg.ServicePort,
				"LocId":   cfg.LocationId,
				"KioskId": cfg.KioskId,
			}

			return c.Render(http.StatusOK, "pub.html", data)
		},
	)
	e.GET(
		"/sub", func(c echo.Context) error {
			data := map[string]interface{}{
				"Port": cfg.ServicePort,
			}

			return c.Render(http.StatusOK, "sub.html", data)
		},
	)
}

func handleShutdown(h *handler.Handler) {
	sig := make(chan os.Signal, 1)
	signal.Notify(sig, os.Interrupt)

	go func() {
		<-sig

		h.Disconnect()
		os.Exit(0)
	}()
}
