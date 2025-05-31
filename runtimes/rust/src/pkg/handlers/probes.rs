use axum::response::IntoResponse;

pub async fn livez() -> impl IntoResponse {
    "OK"
}
