use std::sync::Arc;
use async_nats::{connect, jetstream};
use sqlx::PgPool;
use redis::Client;
use standard_error::StandardError;
use crate::{pkg::conf::settings, prelude::Result};

#[derive(Clone, Debug)]
pub struct AppState {
    pub db_pool: Arc<PgPool>,
    pub redis_client: Arc<Client>,
    pub js: Arc<jetstream::stream::Stream>
}

impl AppState {
    pub async fn new() -> Result<AppState> {
        let db_pool = Arc::new(PgPool::connect(&settings.database_url).await?);
        let redis_client = Arc::new(Client::open(settings.redis_url.as_str()).map_err(|_|StandardError::new("ERR-REDIS-CONN"))?);
        let nc = connect(&settings.nats_broker_url).await.map_err(|_| StandardError::new("ERR-NATS_CONN"))?;
        let stream_name = format!("{}-{}", settings.project, settings.name);
        let subject = format!("{}.{}", settings.project, settings.name);
        let js = Arc::new(jetstream::new(nc).get_or_create_stream(jetstream::stream::Config {
            name: stream_name.clone(),
            subjects: vec![subject.clone()],
            ..Default::default()
        })
        .await
        .map_err(|e| StandardError::new(&format!("ERR-NATS-STREAM: {}", e)))?); 
        Ok(AppState { 
            db_pool,
            redis_client,
            js
        })
    }
}
