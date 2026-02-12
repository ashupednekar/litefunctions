use std::{future::Future, pin::Pin};

use futures::StreamExt;

use super::{conf::settings, state::AppState};
use crate::{
    pkg::function::handler,
    prelude::{Result, natserr},
};

type BoxedConsumer = Pin<Box<dyn Future<Output = Result<()>> + Send>>;

fn consume(state: AppState, mut subscriber: async_nats::Subscriber) -> BoxedConsumer {
    Box::pin(async move {
        let state = state.clone(); //ok, cuz it's a bunch of arcs
        tracing::debug!("waiting for messages...");
        while let Some(msg) = subscriber.next().await {
            tracing::debug!("received event");
            if let Some(req_id) = msg.subject.to_string().split('.').next_back() {
                tracing::debug!("request id: {}", &req_id);
                let (_status, res): (axum::http::StatusCode, Vec<u8>) =
                    handler(state.clone(), Some(req_id), msg.payload.to_vec()).await?;
                tracing::debug!("handler run complete");
                state
                    .nc
                    .publish(
                        format!(
                            "{}.{}.res.rs.{}",
                            &settings.project, &settings.name, &req_id
                        ),
                        res.into(),
                    )
                    .await
                    .map_err(natserr)?;
            }
        } 
        Ok(())
    })
}

pub async fn start_function(state: AppState) -> Result<()> {
    let subject = format!("{}.{}.exec.rs.*", settings.project, settings.name);

    tracing::info!("starting consumer listening to subject: {}", &subject);
    let subscriber = state
        .nc
        .subscribe(subject)
        .await
        .map_err(natserr)?;
    consume(state, subscriber).await?;
    Ok(())
}

#[cfg(test)]
mod tests{
    use std::sync::Arc;

    use standard_error::StandardError;
    use tracing_test::traced_test;

    use super::*;

    #[traced_test]
    #[tokio::test]
    async fn test_consume() -> Result<()>{
        let state = AppState::new().await?;
        start_function(state).await?;
        Ok(())
    }
}
