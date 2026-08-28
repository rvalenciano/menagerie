FROM golang:1.26-alpine AS build
WORKDIR /src
COPY go.mod ./
COPY main.go ./
RUN CGO_ENABLED=0 go build -o /out/bench .

FROM alpine:3.20
COPY --from=build /out/bench /usr/local/bin/bench
ENTRYPOINT ["bench"]
