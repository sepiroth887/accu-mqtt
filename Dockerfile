FROM golang:1.21.5-alpine3.19

COPY ./ /go/github.com/sepiroth887/accu-mqtt

WORKDIR /go/github.com/sepiroth887/accu-mqtt

RUN go build -o /tmp/accu-mqtt .

FROM scratch 

COPY --from=0 /tmp/accu-mqtt /usr/bin/accu-mqtt
COPY --from=0 /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/
COPY ./last_update.json /last_update.json

ENV ACCU_MQTT_BROKER mqtt://127.0.0.1:1883
ENV ACCU_MQTT_API_TOKEN abc-def-hijk
ENV ACCU_MQTT_LATITUDE 37.238
ENV ACCU_MQTT_LONGITUDE -115.804
ENV ACCU_MQTT_TEST_DATA true

CMD ["/usr/bin/accu-mqtt", "start", "-v"]
