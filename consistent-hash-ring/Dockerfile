FROM golang:1.26-alpine AS build
WORKDIR /src
COPY go.mod go.sum ./
COPY cmd ./cmd
COPY internal ./internal
RUN CGO_ENABLED=0 go build -o /out/demo ./cmd/demo

FROM alpine:3.20
RUN apk add --no-cache ca-certificates
COPY --from=build /out/demo /usr/local/bin/demo
ENTRYPOINT ["demo"]
