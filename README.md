# accu-mqtt
Accu Weather Minute Cast MQTT integration for homeassistant

# About
This is an MQTT device/sensor provider for homeassistant to display MinuteCast (current rain state + time before rain starts)
Due to API rate limits (25/day) it will poll the API only once every hour and use the report to update the state and time values based on the last forecast recieved. 
It is currently only allowing for a single location query (using lat/long values provided on startup) and a future version will likely have a local saved cast file to avoid query on restart... tbd

# Usage

## Docker (recommended)
* `docker pull ghcr.io/sepiroth887/accu-mqtt:main`
* Run the image `docker run -it -e ACCU_MQTT_BROKER=mqtt://your-broker-url:port -e ACCU_MQTT_TEST_DATA=true accu-mqtt`
* You should see it sending dummy data
* Setup the API tokens and lat long values using: 
    * ACCU_MQTT_API_TOKEN
    * ACCU_MQTT_LATITUDE
    * ACCU_MQTT_LONGITUDE
* profit

## Golang build env (I assume you know what you are doing)
ensure golang is installed and build it using `go build .` and use the resulting `accu-mqtt` binary (`accu-mqtt --help`)

provide the required inputs (mqtt broker, api token (make sure to select minute cast api in your AccuWeather App) as well as lat/long (x/y flags).

enjoy.

## Error Codes
1 Failed to connect MQTT
2 Failed to register sensors
3 Failed to publish sensor status
