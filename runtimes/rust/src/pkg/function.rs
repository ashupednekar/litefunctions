use crate::{prelude::Result, pkg::state::AppState};
use axum::http::StatusCode;
use uuid::Uuid;

pub async fn handler(_state: AppState, _req_id: Option<&str>, _body: Vec<u8>) -> Result<(StatusCode, Vec<u8>)> {
    tracing::debug!("handler triggered");
    Ok((StatusCode::OK, Uuid::new_v4().to_string().into()))
}
