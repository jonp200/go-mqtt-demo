package client

import (
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"net/url"
	"os"

	mqtt "github.com/eclipse/paho.mqtt.golang"
	"github.com/labstack/gommon/log"
)

type Client struct {
	mqtt.Client
}

func tlsCfg(caName string) *tls.Config {
	certpool := x509.NewCertPool()
	ca, err := os.ReadFile(caName)
	if err != nil {
		log.Fatalf(err.Error())
	}
	certpool.AppendCertsFromPEM(ca)
	return &tls.Config{
		RootCAs: certpool,
	}
}

func publishHandler(_ mqtt.Client, msg mqtt.Message) {
	log.Infof("Received from topic: %v\n>>\t%s", msg.Topic(), msg.Payload())
}

func onConnectAttempt(_ *url.URL, tlsCfg *tls.Config) *tls.Config {
	log.Infof("Connecting to broker...")
	return tlsCfg
}

func onReconnecting(_ mqtt.Client, _ *mqtt.ClientOptions) {
	log.Infof("Reconnecting to broker...")
}

func onConnect(_ mqtt.Client) {
	log.Infof("Connected to broker")
}

func onConnectionLost(_ mqtt.Client, err error) {
	log.Infof("Connection to broker lost: %v", err)
}

func New(caName string) *Client {
	opts := mqtt.NewClientOptions()

	opts.AddBroker(fmt.Sprintf("mqtts://%v:%v", os.Getenv("BROKER_ADDRESS"), os.Getenv("BROKER_PORT")))
	opts.SetTLSConfig(tlsCfg(caName))

	opts.SetClientID(os.Getenv("CLIENT_ID"))
	opts.SetUsername(os.Getenv("CLIENT_USERNAME"))
	opts.SetPassword(os.Getenv("CLIENT_PASSWORD"))

	opts.SetDefaultPublishHandler(publishHandler)
	opts.OnConnectAttempt = onConnectAttempt
	opts.OnReconnecting = onReconnecting
	opts.OnConnect = onConnect
	opts.OnConnectionLost = onConnectionLost

	return &Client{mqtt.NewClient(opts)}
}
