use crate::prelude::Result;

pub async fn livez() -> Result<()> {
    tracing::debug!("service is live");
    Ok(())
}

pub async fn healthz() -> Result<()> {
    tracing::debug!("service is healthy");
    Ok(())
}
