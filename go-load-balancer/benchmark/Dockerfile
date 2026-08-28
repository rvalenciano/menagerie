FROM golang:1.26-alpine AS build
RUN go install github.com/tsenart/vegeta@latest

FROM alpine:3.20
COPY --from=build /go/bin/vegeta /usr/local/bin/vegeta
COPY run-benchmark.sh /usr/local/bin/run-benchmark.sh
RUN chmod +x /usr/local/bin/run-benchmark.sh
ENTRYPOINT ["run-benchmark.sh"]
