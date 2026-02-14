pub mod conf;
pub mod consumer;
pub mod function;
pub mod function_async;
pub mod state;

pub type Result<T> = std::result::Result<T, anyhow::Error>;
