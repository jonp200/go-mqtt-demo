package main

import (
	"encoding/json"
	"net/http"

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

	pubClient := client.NewMqtt("emqxsl-ca.crt")
	if token := pubClient.Connect(); token.Wait() && token.Error() != nil {
		glog.Fatal(token.Error())
	}

	e.File("/", "public/index.html")
	e.File("/favicon.ico", "images/favicon.ico")

	e.POST(
		"/publish", func(c echo.Context) error {
			var p struct {
				Topic string `json:"topic"`
				Data  any    `json:"data"`
			}
			if err := c.Bind(&p); err != nil {
				return echo.NewHTTPError(http.StatusBadRequest, err)
			}

			data, err := json.Marshal(p.Data)
			if err != nil {
				return echo.NewHTTPError(http.StatusInternalServerError, err)
			}

			if token := pubClient.Publish(p.Topic, 0, false, data); token.Wait() && token.Error() != nil {
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
