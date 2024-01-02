# accu-mqtt
Accu Weather Minute Cast MQTT integration for homeassistant

# About
This is an MQTT device/sensor provider for homeassistant to display MinuteCast (current rain state + time before rain starts)
Due to API rate limits (25/day) it will poll the API only once every hour and use the report to update the state and time values based on the last forecast recieved. 
It is currently only allowing for a single location query (using lat/long values provided on startup) and a future version will likely have a local saved cast file to avoid query on restart... tbd

# Usage
Build via docker coming soon(tm)... for now: 

ensure golang is installed and build it using `go build .` and use the resulting `accu-mqtt` binary (`accu-mqtt --help`)

provide the required inputs (mqtt broker, api token (make sure to select minute cast api in your AccuWeather App) as well as lat/long (x/y flags).

enjoy.

## Error Codes
1 Failed to connect MQTT
2 Failed to register sensors
3 Failed to publish sensor status