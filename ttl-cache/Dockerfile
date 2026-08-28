FROM gcc:13-bookworm AS build
WORKDIR /src
COPY Makefile ./
COPY include ./include
COPY src ./src
RUN make build

FROM debian:bookworm-slim
COPY --from=build /src/server /usr/local/bin/server
EXPOSE 6380
ENTRYPOINT ["server"]
