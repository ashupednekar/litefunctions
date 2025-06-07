use async_nats::jetstream::{self, consumer::PullConsumer};
use futures::StreamExt;

use crate::{pkg::function::handler, prelude::{natserr, Result}};
use super::{conf::settings, state::AppState};

pub async fn start_function() -> Result<()> {
    let state = AppState::new().await?;
    let js = &*state.js;

    let consumer_name = format!("{}-{}", settings.project, settings.name);
    let subject = format!("{}.{}.exec.>", settings.project, settings.name);

    let consumer: PullConsumer = js
        .get_or_create_consumer(
            &consumer_name,
            jetstream::consumer::pull::Config {
                durable_name: Some(consumer_name.clone()),
                filter_subject: subject,
                ..Default::default()
            },
        )
        .await
        .map_err(natserr)?;

    let mut msgs = consumer.messages().await.map_err(natserr)?;

    if let Some(Ok(msg)) = msgs.next().await {
        tracing::info!("Received event: {:?}", msg);
        handler(state).await?;
        msg.ack().await.map_err(natserr)?;
    }

    Ok(())
}
