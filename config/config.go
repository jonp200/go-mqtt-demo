package config

import (
	"errors"
	"fmt"
	"net"
	"os"
	"regexp"
	"strconv"
	"time"
)

type Config struct {
	BrokerAddress        string
	BrokerPort           string
	BrokerWSPort         string
	ClientIDSuffix       string
	MQTTUsername         string
	MQTTPassword         string
	MQTTCleanSession     bool
	MaxReconnectInterval time.Duration
	ServicePort          string
	LocationId           string
	KioskId              string
}

func New() (*Config, error) {
	maxReconnectInterval, err := time.ParseDuration(os.Getenv("MQTT_MAX_RECONNECT_INTERVAL"))
	if err != nil {
		return nil, fmt.Errorf("invalid max reconnect interval: %w", err)
	}

	cfg := &Config{
		BrokerAddress:        os.Getenv("BROKER_ADDRESS"),
		BrokerPort:           os.Getenv("BROKER_PORT"),
		BrokerWSPort:         os.Getenv("BROKER_WS_PORT"),
		ClientIDSuffix:       os.Getenv("CLIENT_ID_SUFFIX"),
		MQTTUsername:         os.Getenv("MQTT_USERNAME"),
		MQTTPassword:         os.Getenv("MQTT_PASSWORD"),
		MQTTCleanSession:     os.Getenv("MQTT_CLEAN_SESSION") == "1",
		MaxReconnectInterval: maxReconnectInterval,
		ServicePort:          os.Getenv("SERVICE_PORT"),
		LocationId:           os.Getenv("LOCATION_ID"),
		KioskId:              os.Getenv("KIOSK_ID"),
	}

	if err := cfg.Validate(); err != nil {
		return nil, err
	}

	return cfg, nil
}

func (c *Config) Validate() error {
	if err := c.validateEmpty(); err != nil {
		return err
	}

	if err := c.validateBrokerAddress(); err != nil {
		return err
	}

	return c.validatePorts()
}

func (c *Config) validateEmpty() error {
	if c.BrokerAddress == "" {
		return errors.New("broker address is required")
	}

	if c.BrokerPort == "" {
		return errors.New("broker port is required")
	}

	if c.BrokerWSPort == "" {
		return errors.New("broker websocket port is required")
	}

	if c.ClientIDSuffix == "" {
		return errors.New("client id suffix is required")
	}

	if c.ServicePort == "" {
		return errors.New("service port is required")
	}

	if c.LocationId == "" {
		return errors.New("location id is required")
	}

	return nil
}

func (c *Config) validateBrokerAddress() error {
	if net.ParseIP(c.BrokerAddress) == nil && !isValidDomain(c.BrokerAddress) {
		return errors.New("broker address must be a valid ip address or domain name")
	}

	return nil
}

func isValidDomain(domain string) bool {
	// Regular expression to validate domain name
	regex := `^(?:[a-zA-Z0-9](?:[a-zA-Z0-9-]{0,61}[a-zA-Z0-9])?\.)+[a-zA-Z]{2,}$`

	match, err := regexp.MatchString(regex, domain)
	if err != nil {
		return false
	}

	return match
}

const (
	MinPort = 1
	MaxPort = 65535
)

type PortError struct {
	Port    string
	Name    string
	Wrapped error
}

func (e *PortError) Error() string {
	return fmt.Sprintf("invalid %s port %q: %v", e.Name, e.Port, e.Wrapped)
}

func (e *PortError) Unwrap() error {
	return e.Wrapped
}

func (c *Config) validatePorts() error {
	ports := map[string]string{
		"broker":    c.BrokerPort,
		"websocket": c.BrokerWSPort,
		"service":   c.ServicePort,
	}

	for name, port := range ports {
		if err := validatePort(port); err != nil {
			return &PortError{
				Port:    port,
				Name:    name,
				Wrapped: err,
			}
		}
	}

	return nil
}

func validatePort(port string) error {
	num, err := strconv.ParseInt(port, 10, 64)
	if err != nil {
		return fmt.Errorf("invalid port number: %w", err)
	}

	if num < MinPort || num > MaxPort {
		return fmt.Errorf("port number %d out of range (1-65535)", num)
	}

	return nil
}
