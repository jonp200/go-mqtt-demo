package main

import (
	"fmt"
	"time"

	mqtt "github.com/eclipse/paho.mqtt.golang"
	"github.com/joho/godotenv"
	glog "github.com/labstack/gommon/log"
	"go-mqtt-demo/client"
	"go-mqtt-demo/logger"
)

func main() {
	if err := godotenv.Load(); err != nil {
		glog.Fatal("Error loading .env file")
	}

	logger.Init()

	c := client.NewMqtt("emqxsl-ca.crt", "emqx_demo")
	if token := c.Connect(); token.Wait() && token.Error() != nil {
		glog.Fatal(token.Error())
	}

	func(client mqtt.Client) {
		{
			// Sample subscribe message with the topic `location/1/kiosk/1/sensor/1`
			// Ideally, a different client subscribes with this topic.
			topic := "location/1/kiosk/1/sensor/1"
			token := client.Subscribe(topic, 1, nil)
			token.Wait()
			glog.Infof("Subscribed to topic %s", topic)
		}

		topic := "location/1/kiosk/config"
		token := client.Subscribe(topic, 1, nil)
		token.Wait()
		glog.Infof("Subscribed to topic %s", topic)
	}(c)

	func(client mqtt.Client) {
		{
			// Sample publish message with the topic `location/1/kiosk/config`.
			// Ideally, a different client publishes with this topic.
			token := client.Publish("location/1/kiosk/config", 0, false, `{"enabled":true}`)
			token.Wait()
		}

		const num = 10
		for i := 0; i < num; i++ {
			text := fmt.Sprintf("Message %d", i)
			token := client.Publish("location/1/kiosk/1/sensor/1", 0, false, text)
			token.Wait()
			time.Sleep(time.Second)
		}
	}(c)

	const wait = 250

	c.Disconnect(wait)
}
