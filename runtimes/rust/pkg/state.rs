use crate::pkg::conf::load_settings;
use crate::pkg::Result;
use redis::AsyncCommands;
use sqlx::PgPool;
use url::Url;

#[derive(Clone)]
pub struct AppState {
    pub db_pool: PgPool,
    pub redis_client: redis::Client,
    pub nc: async_nats::Client,
}

pub async fn new_app_state() -> Result<AppState> {
    let settings = load_settings();
    validate_settings(settings)?;

    tracing::info!(
        "settings: project={} name={} nats_url={} database_url_set={} redis_url_set={}",
        settings.project,
        settings.name,
        safe_url_summary(&settings.nats_url),
        !settings.database_url.is_empty(),
        !settings.redis_url.is_empty()
    );

    let db_pool = PgPool::connect(&settings.database_url).await?;

    let redis_url = redis_url_with_password(settings);
    let redis_client = redis::Client::open(redis_url.as_str())?;
    let mut redis_conn = redis_client.get_async_connection().await?;
    let _: String = redis_conn.ping().await?;
    let _: String = redis_conn.ping().await?;

    let nc = async_nats::connect(&settings.nats_url).await?;

    Ok(AppState {
        db_pool,
        redis_client,
        nc,
    })
}

fn validate_settings(settings: &crate::pkg::conf::Settings) -> Result<()> {
    if settings.project.is_empty() {
        anyhow::bail!("ERR-SETTINGS: PROJECT is required (set env PROJECT)");
    }
    if settings.name.is_empty() {
        anyhow::bail!("ERR-SETTINGS: NAME is required (set env NAME)");
    }
    if settings.database_url.is_empty() {
        anyhow::bail!("ERR-SETTINGS: DATABASE_URL is required (set env DATABASE_URL)");
    }
    if settings.redis_url.is_empty() {
        anyhow::bail!("ERR-SETTINGS: REDIS_URL is required (set env REDIS_URL)");
    }
    if settings.nats_url.is_empty() {
        anyhow::bail!("ERR-SETTINGS: NATS_URL is required (set env NATS_URL)");
    }
    Ok(())
}

fn redis_url_with_password(settings: &crate::pkg::conf::Settings) -> String {
    if settings.redis_password.is_empty() {
        return settings.redis_url.clone();
    }
    if settings.redis_url.contains('@') {
        return settings.redis_url.clone();
    }

    if let Some(rest) = settings.redis_url.strip_prefix("redis://") {
        return format!("redis://:{}@{}", settings.redis_password, rest);
    }
    if let Some(rest) = settings.redis_url.strip_prefix("rediss://") {
        return format!("rediss://:{}@{}", settings.redis_password, rest);
    }

    settings.redis_url.clone()
}

fn safe_url_summary(raw: &str) -> String {
    if raw.trim().is_empty() {
        return "(empty)".to_string();
    }
    let url = match Url::parse(raw) {
        Ok(url) => url,
        Err(_) => return "(invalid)".to_string(),
    };
    let host = match url.host_str() {
        Some(host) => host,
        None => return "(invalid)".to_string(),
    };
    format!("{}://{}", url.scheme(), host)
}
