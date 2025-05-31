use crate::{pkg::{conf::settings, routes::build_routes}, prelude::Result};
use clap::{Parser, Subcommand};
use tokio::net::TcpListener;

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
            let listener = TcpListener::bind(&format!("0.0.0.0:{}", &settings.listen_port))
                .await
                .unwrap();
            tracing::info!("listening at: {}", &settings.listen_port);
            axum::serve(listener, build_routes().await?).await?;
        }
        None => {
            tracing::error!("no subcommand passed")
        }
    }
    Ok(())
}
