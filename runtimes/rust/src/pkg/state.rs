use crate::{pkg::conf::settings, prelude::Result};
use async_nats::connect;
use sqlx::PgPool;
use standard_error::StandardError;
use std::sync::Arc;
use tracing::info;

#[derive(Clone, Debug)]
pub struct AppState {
    pub db_pool: Arc<PgPool>,
    pub redis_client: Arc<redis::Client>,
    pub nc: Arc<async_nats::Client>,
}

impl AppState {
    pub async fn new() -> Result<AppState> {
        validate_settings()?;
        info!(
            "settings: project={} name={} nats_url={} database_url_set={} redis_url_set={}",
            settings.project,
            settings.name,
            safe_url_summary(&settings.nats_url),
            !settings.database_url.is_empty(),
            !settings.redis_url.is_empty()
        );
        let db_pool = Arc::new(PgPool::connect(&settings.database_url).await?);
        let redis_url = redis_url_with_password();
        let redis_client = Arc::new(
            redis::Client::open(redis_url.as_str())
                .map_err(|_| StandardError::new("ERR-REDIS-CONN"))?,
        );
        let nc = connect(&settings.nats_url)
            .await
            .map_err(|_| StandardError::new("ERR-NATS_CONN"))?;
        let nc = Arc::new(nc);
        Ok(AppState {
            db_pool,
            redis_client,
            nc,
        })
    }
}

fn validate_settings() -> Result<()> {
    if settings.project.trim().is_empty() {
        return Err(StandardError::new(
            "ERR-SETTINGS: PROJECT is required (set env PROJECT)",
        ));
    }
    if settings.name.trim().is_empty() {
        return Err(StandardError::new("ERR-SETTINGS: NAME is required (set env NAME)"));
    }
    if settings.database_url.trim().is_empty() {
        return Err(StandardError::new(
            "ERR-SETTINGS: DATABASE_URL is required (set env DATABASE_URL)",
        ));
    }
    if settings.redis_url.trim().is_empty() {
        return Err(StandardError::new(
            "ERR-SETTINGS: REDIS_URL is required (set env REDIS_URL)",
        ));
    }
    if settings.nats_url.trim().is_empty() {
        return Err(StandardError::new(
            "ERR-SETTINGS: NATS_URL is required (set env NATS_URL)",
        ));
    }
    Ok(())
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

fn safe_url_summary(raw: &str) -> String {
    if raw.trim().is_empty() {
        return "(empty)".to_string();
    }
    let mut parts = raw.split("://");
    let scheme = parts.next().unwrap_or("");
    let rest = parts.next().unwrap_or("");
    if scheme.is_empty() || rest.is_empty() {
        return "(invalid)".to_string();
    }
    let host_part = rest.split('/').next().unwrap_or("");
    let host = host_part.split('@').last().unwrap_or("");
    if host.is_empty() {
        return "(invalid)".to_string();
    }
    format!("{}://{}", scheme, host)
}
