package main

import (
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"log"
	"net/url"
	"os"
	"time"

	mqtt "github.com/eclipse/paho.mqtt.golang"
	"github.com/joho/godotenv"
)

func setupLogging() {
	mqtt.ERROR = log.New(os.Stderr, "ERROR ", log.LstdFlags)
	mqtt.CRITICAL = log.New(os.Stderr, "CRITICAL ", log.LstdFlags)
	// mqtt.WARN = log.New(os.Stderr, "WARN ", log.LstdFlags)
	// mqtt.DEBUG = log.New(os.Stdout, "DEBUG ", log.LstdFlags)
}

func tlsCfg() *tls.Config {
	certpool := x509.NewCertPool()
	ca, err := os.ReadFile("emqxsl-ca.crt")
	if err != nil {
		log.Fatalln(err.Error())
	}
	certpool.AppendCertsFromPEM(ca)
	return &tls.Config{
		RootCAs: certpool,
	}
}

func sub(client mqtt.Client) {
	{
		// Sample subscribe message with the topic `location/1/kiosk/1/sensor1`
		// Ideally, a different client subscribes with this topic.
		topic := "location/1/kiosk/1/sensor1"
		token := client.Subscribe(topic, 1, nil)
		token.Wait()
		fmt.Printf("Subscribed to topic %s\n", topic)
	}

	topic := "location/1/kiosk/config"
	token := client.Subscribe(topic, 1, nil)
	token.Wait()
	fmt.Printf("Subscribed to topic %s\n", topic)
}

func publish(client mqtt.Client) {
	{
		// Sample publish message with the topic `location/1/kiosk/config`.
		// Ideally, a different client publishes with this topic.
		token := client.Publish("location/1/kiosk/config", 0, false, `{"enabled":true}`)
		token.Wait()
	}

	num := 10
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

	opts := mqtt.NewClientOptions()
	opts.AddBroker(fmt.Sprintf("mqtts://%v:%v", os.Getenv("BROKER_ADDRESS"), os.Getenv("BROKER_PORT")))
	opts.SetTLSConfig(tlsCfg())
	opts.SetClientID(os.Getenv("CLIENT_ID"))
	opts.SetUsername(os.Getenv("CLIENT_USERNAME"))
	opts.SetPassword(os.Getenv("CLIENT_PASSWORD"))

	opts.SetDefaultPublishHandler(
		func(_ mqtt.Client, msg mqtt.Message) {
			fmt.Printf("Received from topic: %v\n>>\t%s\n", msg.Topic(), msg.Payload())
		},
	)
	opts.OnConnectAttempt = func(_ *url.URL, tlsCfg *tls.Config) *tls.Config {
		fmt.Println("Connecting...")
		return tlsCfg
	}
	opts.OnReconnecting = func(_ mqtt.Client, _ *mqtt.ClientOptions) {
		fmt.Println("Reconnecting...")
	}
	opts.OnConnect = func(_ mqtt.Client) {
		fmt.Println("Connected")
	}
	opts.OnConnectionLost = func(_ mqtt.Client, err error) {
		fmt.Printf("Connection lost: %v", err)
	}

	client := mqtt.NewClient(opts)
	if token := client.Connect(); token.Wait() && token.Error() != nil {
		log.Fatal(token.Error())
	}

	sub(client)
	publish(client)

	client.Disconnect(250)
}
