use axum::{Json, http::StatusCode};
use serde::{Deserialize, Serialize};

#[derive(Deserialize)]
pub struct Input {
    pub name: Option<String>,
}

#[derive(Serialize)]
pub struct Output {
    pub message: String,
}

pub async fn handle(Json(input): Json<Input>) -> (StatusCode, Json<Output>) {
    let name = input.name.unwrap_or("stranger".into());
    (
        StatusCode::OK,
        Json(Output {
            message: format!("Hello {} from Rust!", name),
        }),
    )
}
