pub mod handlers;
pub mod router;
pub mod state;

use crate::{conf::settings, prelude::Result};
use router::build_routes;

pub async fn listen() -> Result<()> {
    let listener =
        tokio::net::TcpListener::bind(format!("0.0.0.0:{}", settings.listen_port.clone())).await?;
    tracing::info!("Listening at port {}", settings.listen_port);
    tokio::select! {
        r = axum::serve(listener, build_routes().await?) => {
            tracing::warn!("server ended unexpectedly: {:?}", &r)
        },
        _ = tokio::signal::ctrl_c() => {
            tracing::info!("received ctrl+c interrupt, closing server");
        }
    }
    Ok(())
}
