use crate::{pkg::consumer::start_function, prelude::Result};
use clap::{Parser, Subcommand};

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
            start_function().await?;
        }
        None => {
            tracing::error!("no subcommand passed")
        }
    }
    Ok(())
}
