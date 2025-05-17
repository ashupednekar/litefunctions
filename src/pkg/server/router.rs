use axum::{Router, routing::get};

use super::handlers::probes::{healthz, livez};
use super::state::AppState;
use crate::prelude::Result;

pub async fn build_routes() -> Result<Router> {
    let state = AppState::new().await?;
    let app = Router::new()
        .route("/healthz", get(healthz))
        .route("/livez", get(livez))
        .with_state(state);

    Ok(app)
}
