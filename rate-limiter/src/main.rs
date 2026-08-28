//! Demo HTTP server: an Axum app with a middleware layer that applies a
//! single, process-wide `TokenBucket` to every request, returning 429
//! once it's empty.

use std::sync::Arc;

use axum::{
    body::Body,
    extract::{Request, State},
    http::StatusCode,
    middleware::{self, Next},
    response::Response,
    routing::get,
    Router,
};

use rate_limiter::token_bucket::TokenBucket;

type SharedBucket = Arc<TokenBucket>;

const CAPACITY: u32 = 10;
const REFILL_RATE_PER_SEC: f64 = 5.0;

async fn rate_limit(State(bucket): State<SharedBucket>, request: Request, next: Next) -> Response {
    if bucket.try_acquire() {
        next.run(request).await
    } else {
        Response::builder()
            .status(StatusCode::TOO_MANY_REQUESTS)
            .body(Body::from("rate limit exceeded\n"))
            .expect("building a static response should never fail")
    }
}

async fn handler() -> &'static str {
    "ok\n"
}

#[tokio::main]
async fn main() {
    let bucket: SharedBucket = Arc::new(TokenBucket::new(CAPACITY, REFILL_RATE_PER_SEC));

    let app = Router::new()
        .route("/", get(handler))
        .layer(middleware::from_fn_with_state(bucket, rate_limit));

    let listener = tokio::net::TcpListener::bind("0.0.0.0:3000")
        .await
        .expect("failed to bind to 0.0.0.0:3000");

    println!("rate-limiter demo listening on 0.0.0.0:3000");
    axum::serve(listener, app)
        .await
        .expect("server error");
}
