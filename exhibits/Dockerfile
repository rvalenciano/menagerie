FROM golang:1.26-alpine AS build
WORKDIR /src
COPY go.mod ./
COPY cmd ./cmd
RUN CGO_ENABLED=0 go build -o /out/backend ./cmd/backend

FROM alpine:3.20
COPY --from=build /out/backend /usr/local/bin/backend
EXPOSE 8080
ENTRYPOINT ["backend"]
