use crate::{pkg::server::listen, prelude::Result};
use clap::{Parser, Subcommand};

#[derive(Parser)]
#[command(about = "starts lite web services")]
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
            listen().await?;
        }
        None => {
            tracing::error!("no subcommand passed");
        }
    }
    Ok(())
}
