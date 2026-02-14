#[path = "../pkg/mod.rs"]
mod pkg;

use pkg::conf::load_settings;
use pkg::consumer::start_function;
use pkg::state::new_app_state;
use std::net::SocketAddr;
use tracing::{error, info};

#[tokio::main]
async fn main() {
    tracing_subscriber::fmt::init();

    let settings = load_settings();
    let span = tracing::info_span!("runtime", project = %settings.project, function = %settings.name);
    let _enter = span.enter();

    let state = match new_app_state().await {
        Ok(state) => state,
        Err(err) => {
            error!(error = %err, "failed to initialize runtime state");
            return;
        }
    };

    let http_state = state.clone();
    let http_settings = settings.clone();
    tokio::spawn(async move {
        start_http_server(http_state, &http_settings).await;
    });

    if let Err(err) = start_function(state).await {
        error!(error = %err, "function consumer exited");
    }
}

async fn start_http_server(_state: pkg::state::AppState, settings: &pkg::conf::Settings) {
    let port = if settings.http_port.is_empty() {
        "8080".to_string()
    } else {
        settings.http_port.clone()
    };

    let addr: SocketAddr = match format!("0.0.0.0:{}", port).parse() {
        Ok(addr) => addr,
        Err(err) => {
            error!(error = %err, "invalid http port");
            return;
        }
    };

    let app = axum::Router::new().route("/", axum::routing::get(pkg::function::handle));

    info!(addr = %addr, "starting http server");
    let listener = match tokio::net::TcpListener::bind(addr).await {
        Ok(listener) => listener,
        Err(err) => {
            error!(error = %err, "http server bind error");
            return;
        }
    };

    if let Err(err) = axum::serve(listener, app).await {
        error!(error = %err, "http server error");
    }
}
