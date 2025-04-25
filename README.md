# MQTT demo with Go

## Environment variables

| Key                           | Description                              |
|-------------------------------|------------------------------------------|
| `BROKER_ADDRESS`              | Broker or server address                 |
| `BROKER_MQTT_PORT`            | Broker or server MQTT TLS/SSL port       |
| `BROKER_WS_PORT`              | Broker or server WebSocket TLS/SSL port  |
| `CLIENT_ID_SUFFIX`            | Unique client ID suffix                  |
| `MQTT_USERNAME`               |                                          |
| `MQTT_PASSWORD`               |                                          |
| `MQTT_CLEAN_SESSION`          | Value is `0` or `1`                      |
| `MQTT_MAX_RECONNECT_INTERVAL` | Default `10s`                            |
| `SERVICE_PORT`                | Port which the service will be bind to   |
| `LOCATION_ID`                 | Location identifier                      |
| `KIOSK_ID`                    | Kiosk identifier                         |
| `DEBUG`                       | Log with debug mode. Value is `0` or `1` |

## Commands

### Admin

Example `.env` file:

```dotenv
BROKER_ADDRESS=broker.emqx.io
BROKER_PORT=8883
BROKER_WS_PORT=8084
CLIENT_ID_SUFFIX=admin1
MQTT_USERNAME=emqx
MQTT_PASSWORD=public
MQTT_CLEAN_SESSION=0
MQTT_MAX_RECONNECT_INTERVAL=10s
SERVICE_PORT=8080
LOCATION_ID=1
DEBUG=0
```

Run command:

```shell
cd admin
go run .
```

For succeeding instances (through separate terminals):

```shell
cd admin
SERVICE_PORT=7979 LOCATION_ID=2 CLIENT_ID_SUFFIX=admin2 go run .
```

### Kiosk

Example `.env` file:

```dotenv
BROKER_ADDRESS=broker.emqx.io
BROKER_PORT=8883
BROKER_WS_PORT=8084
CLIENT_ID_SUFFIX=loc1_kiosk1
MQTT_USERNAME=emqx
MQTT_PASSWORD=public
MQTT_CLEAN_SESSION=0
MQTT_MAX_RECONNECT_INTERVAL=10s
SERVICE_PORT=8181
LOCATION_ID=1
KIOSK_ID=1
DEBUG=0
```

Run command:

```shell
cd kiosk
go run .
```

For succeeding instances (through separate terminals):

```shell
cd kiosk
SERVICE_PORT=8282 CLIENT_ID_SUFFIX=loc1_kiosk2 KIOSK_ID=2 go run .
```

```shell
cd kiosk
SERVICE_PORT=8383 CLIENT_ID_SUFFIX=loc2_kiosk1 LOCATION_ID=2 KIOSK_ID=1 go run .
```