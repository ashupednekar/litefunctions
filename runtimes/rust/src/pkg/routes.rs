use axum::{routing::get, Router};

use crate::prelude::Result;
use super::{handlers::probes::livez, state::AppState};

pub async fn build_routes() -> Result<Router> {
    let state = AppState::new().await?;
    Ok(Router::new()
        .route("/livez/", get(livez))
        .with_state(state))
}
