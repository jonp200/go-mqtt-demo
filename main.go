package main

import (
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"log"
	"os"
	"time"

	mqtt "github.com/eclipse/paho.mqtt.golang"
	"github.com/joho/godotenv"
)

func setupLogging() {
	mqtt.ERROR = log.New(os.Stderr, "ERROR ", log.LstdFlags)
	mqtt.CRITICAL = log.New(os.Stderr, "CRITICAL ", log.LstdFlags)
	mqtt.WARN = log.New(os.Stderr, "WARN ", log.LstdFlags)
	// mqtt.DEBUG = log.New(os.Stdout, "DEBUG ", log.LstdFlags)
}

func tlsConfig() *tls.Config {
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
	topic := "location/1/kiosk/config"
	token := client.Subscribe(topic, 1, nil)
	token.Wait()
	fmt.Printf("Subscribed to topic %s", topic)
}

func publish(client mqtt.Client) {
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

	var broker = os.Getenv("BROKER_ADDRESS")
	var port = os.Getenv("BROKER_PORT")
	opts := mqtt.NewClientOptions()
	opts.AddBroker(fmt.Sprintf("mqtts://%v:%v", broker, port))
	opts.SetTLSConfig(tlsConfig())
	opts.SetClientID(os.Getenv("CLIENT_ID"))
	opts.SetUsername(os.Getenv("CLIENT_USERNAME"))
	opts.SetPassword(os.Getenv("CLIENT_PASSWORD"))

	opts.SetDefaultPublishHandler(
		func(client mqtt.Client, msg mqtt.Message) {
			fmt.Printf("Received message: %s from topic: %s\n", msg.Payload(), msg.Topic())
		},
	)
	opts.OnConnect = func(client mqtt.Client) {
		fmt.Println("Connected")
	}
	opts.OnConnectionLost = func(client mqtt.Client, err error) {
		fmt.Printf("Connect lost: %v", err)
	}

	client := mqtt.NewClient(opts)
	if token := client.Connect(); token.Wait() && token.Error() != nil {
		panic(token.Error())
	}

	sub(client)
	publish(client)

	client.Disconnect(250)
}
