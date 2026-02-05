use crate::{pkg::conf::settings, prelude::Result};
use async_nats::{connect, jetstream};
use sqlx::PgPool;
use standard_error::StandardError;
use std::sync::Arc;

#[derive(Clone, Debug)]
pub struct AppState {
    pub db_pool: Arc<PgPool>,
    pub redis_client: Arc<redis::Client>,
    pub nc: Arc<async_nats::Client>,
    pub js: Arc<jetstream::stream::Stream>,
}

impl AppState {
    pub async fn new() -> Result<AppState> {
        let db_pool = Arc::new(PgPool::connect(&settings.database_url).await?);
        let redis_url = redis_url_with_password();
        let redis_client = Arc::new(
            redis::Client::open(redis_url.as_str())
                .map_err(|_| StandardError::new("ERR-REDIS-CONN"))?,
        );
        let nc = connect(&settings.nats_url)
            .await
            .map_err(|_| StandardError::new("ERR-NATS_CONN"))?;
        let config = jetstream::stream::Config {
            name: settings.project.clone(),
            subjects: vec![format!("{}.>", &settings.project)],
            ..Default::default()
        };
        let js = Arc::new(
            jetstream::new(nc.clone())
                .get_or_create_stream(config)
                .await
                .map_err(|e| StandardError::new(&format!("ERR-NATS-STREAM: {}", e)))?,
        );
        let nc = Arc::new(nc);
        Ok(AppState {
            db_pool,
            redis_client,
            nc,
            js,
        })
    }
}

fn redis_url_with_password() -> String {
    let password = match settings.redis_password.as_deref() {
        Some(value) if !value.is_empty() => value,
        _ => return settings.redis_url.clone(),
    };

    if settings.redis_url.contains('@') {
        return settings.redis_url.clone();
    }

    if let Some(rest) = settings.redis_url.strip_prefix("redis://") {
        return format!("redis://:{}@{}", password, rest);
    }

    if let Some(rest) = settings.redis_url.strip_prefix("rediss://") {
        return format!("rediss://:{}@{}", password, rest);
    }

    settings.redis_url.clone()
}
