FROM golang:1.26-alpine AS build
WORKDIR /src
COPY go.mod ./
COPY cmd ./cmd
COPY internal ./internal
RUN CGO_ENABLED=0 go build -o /out/rate-limiter ./cmd/server

FROM alpine:3.20
RUN apk add --no-cache ca-certificates wget
COPY --from=build /out/rate-limiter /usr/local/bin/rate-limiter
EXPOSE 3000
ENTRYPOINT ["rate-limiter"]
