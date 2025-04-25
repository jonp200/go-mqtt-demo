package client

import (
	"crypto/tls"
	"crypto/x509"
	"net/url"
	"os"
	"time"

	mqtt "github.com/eclipse/paho.mqtt.golang"
	glog "github.com/labstack/gommon/log"
)

const (
	DefaultQuiesceTimeout = 250
	DefaultBufferSize     = 256
)

func setTLSConfig(opts *mqtt.ClientOptions, caName string) (*tls.Config, error) {
	certpool := x509.NewCertPool()

	ca, err := os.ReadFile(caName)
	if err != nil {
		return nil, err
	}

	certpool.AppendCertsFromPEM(ca)

	tc := &tls.Config{RootCAs: certpool}
	opts.SetTLSConfig(tc)

	return tc, nil
}

func setAuth(clientId string, opts *mqtt.ClientOptions) {
	// The resulting client ID must be unique, otherwise the broker will reject the connection
	opts.SetClientID(clientId + "_" + os.Getenv("CLIENT_ID_SUFFIX"))
	opts.SetUsername(os.Getenv("MQTT_USERNAME"))
	opts.SetPassword(os.Getenv("MQTT_PASSWORD"))
}

func setCleanSession(opts *mqtt.ClientOptions) {
	opts.CleanSession = os.Getenv("MQTT_CLEAN_SESSION") == "1"
}

func setReconnect(opts *mqtt.ClientOptions) {
	opts.SetAutoReconnect(true)

	maxReconnectInterval, _ := time.ParseDuration(os.Getenv("MQTT_MAX_RECONNECT_INTERVAL"))
	opts.SetMaxReconnectInterval(maxReconnectInterval)
}

func defaultPublishHandler(_ mqtt.Client, msg mqtt.Message) {
	glog.Infof("received from topic: %v\n>>\t%s", msg.Topic(), msg.Payload())
}

func onConnectAttempt(broker *url.URL, tlsCfg *tls.Config) *tls.Config {
	glog.Infof("connecting to broker %s...", broker.Host)

	return tlsCfg
}

func onReconnecting(_ mqtt.Client, opts *mqtt.ClientOptions) {
	glog.Infof("reconnecting to broker %s...", opts.Servers)
}

func onConnect(_ mqtt.Client) {
	glog.Info("connected to broker")
}

func onConnectionLost(_ mqtt.Client, err error) {
	glog.Infof("connection to broker lost: %v", err)
}
