FROM rust:1-slim AS build
WORKDIR /src
COPY Cargo.toml ./
COPY src ./src
RUN cargo build --release

FROM debian:bookworm-slim
RUN apt-get update && apt-get install -y --no-install-recommends ca-certificates wget \
    && rm -rf /var/lib/apt/lists/*
COPY --from=build /src/target/release/rate-limiter /usr/local/bin/rate-limiter
EXPOSE 3000
ENTRYPOINT ["rate-limiter"]
