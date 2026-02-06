use crate::{
    pkg::consumer::start_function,
    pkg::conf::settings,
    pkg::function::handler,
    pkg::state::AppState,
    prelude::Result,
};
use axum::{body::Bytes, extract::State, http::StatusCode, response::IntoResponse, routing::any, Router};
use clap::{Parser, Subcommand};
use std::net::SocketAddr;

#[derive(Parser)]
#[command(about = "lets you run auth-svc commands")]
struct Cmd {
    #[command(subcommand)]
    command: Option<SubCommandType>,
}

#[derive(Subcommand)]
enum SubCommandType {
    Listen,
}

pub async fn run() -> Result<()> {
    let args = Cmd::parse();
    match args.command {
        Some(SubCommandType::Listen) => {
            let state = AppState::new().await?;
            let http_state = state.clone();
            tokio::spawn(async move {
                if let Err(err) = start_http_server(http_state).await {
                    tracing::error!("http server error: {}", err);
                }
            });
            start_function(state).await?;
        }
        None => {
            tracing::error!("no subcommand passed")
        }
    }
    Ok(())
}

async fn start_http_server(state: AppState) -> Result<()> {
    let port = settings.http_port.unwrap_or(8080);
    let app = Router::new()
        .route("/*path", any(handle_request))
        .with_state(state);

    let addr: SocketAddr = ([0, 0, 0, 0], port).into();
    tracing::info!("http server listening on {}", addr);
    let listener = tokio::net::TcpListener::bind(addr).await?;
    axum::serve(listener, app).await?;
    Ok(())
}

async fn handle_request(State(state): State<AppState>, body: Bytes) -> impl IntoResponse {
    match handler(state, None, body.to_vec()).await {
        Ok((status, res)) => (status, res).into_response(),
        Err(err) => (StatusCode::INTERNAL_SERVER_ERROR, err.to_string()).into_response(),
    }
}
