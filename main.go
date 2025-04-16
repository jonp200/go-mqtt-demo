package main

import (
	"fmt"
	"log"
	"os"
	"time"

	mqtt "github.com/eclipse/paho.mqtt.golang"
	"github.com/joho/godotenv"
	glog "github.com/labstack/gommon/log"
	"go-mqtt-demo/client"
	"go-mqtt-demo/logger"
)

func setupLogging() {
	glog.SetHeader("${time_rfc3339} ${level} ${short_file}:${line}")
	glog.EnableColor()

	mqtt.ERROR = logger.Error{}
	mqtt.CRITICAL = logger.Error{}
	mqtt.WARN = logger.Warn{}

	if os.Getenv("DEBUG") == "1" {
		glog.SetLevel(glog.DEBUG)
		mqtt.DEBUG = logger.Debug{}
	}
}

func sub(client mqtt.Client) {
	{
		// Sample subscribe message with the topic `location/1/kiosk/1/sensor1`
		// Ideally, a different client subscribes with this topic.
		topic := "location/1/kiosk/1/sensor1"
		token := client.Subscribe(topic, 1, nil)
		token.Wait()
		glog.Infof("Subscribed to topic %s", topic)
	}

	topic := "location/1/kiosk/config"
	token := client.Subscribe(topic, 1, nil)
	token.Wait()
	glog.Infof("Subscribed to topic %s", topic)
}

func publish(client mqtt.Client) {
	{
		// Sample publish message with the topic `location/1/kiosk/config`.
		// Ideally, a different client publishes with this topic.
		token := client.Publish("location/1/kiosk/config", 0, false, `{"enabled":true}`)
		token.Wait()
	}

	const num = 10
	for i := 0; i < num; i++ {
		text := fmt.Sprintf("Message %d", i)
		token := client.Publish("location/1/kiosk/1/sensor1", 0, false, text)
		token.Wait()
		time.Sleep(time.Second)
	}
}

func main() {
	if err := godotenv.Load(); err != nil {
		log.Fatal("Error loading .env file")
	}
	setupLogging()

	c := client.New("emqxsl-ca.crt")
	if token := c.Connect(); token.Wait() && token.Error() != nil {
		log.Fatal(token.Error())
	}

	sub(c)
	publish(c)

	c.Disconnect(250)
}
