use std::sync::Arc;
use sqlx::PgPool;
use redis::Client;
use standard_error::StandardError;
use crate::{pkg::conf::settings, prelude::Result};

#[derive(Clone, Debug)]
pub struct AppState {
    pub db_pool: Arc<PgPool>,
    pub redis_client: Arc<Client>,
}

impl AppState {
    pub async fn new() -> Result<AppState> {
        let db_pool = Arc::new(PgPool::connect(&settings.database_url).await?);
        let redis_client = Arc::new(Client::open(settings.redis_url.as_str()).map_err(|_|StandardError::new("ERR-REDIS-CONN"))?);
        Ok(AppState { 
            db_pool,
            redis_client
        })
    }
}
