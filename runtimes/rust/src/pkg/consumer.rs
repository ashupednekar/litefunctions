use std::{pin::Pin, time::Duration};

use async_nats::jetstream::{self, consumer::PullConsumer, consumer::pull::Stream};
use futures::StreamExt;
use standard_error::StandardError;

use super::{conf::settings, state::AppState};
use crate::{
    pkg::function::handler,
    prelude::{Result, natserr},
};

type BoxedConsumer<'a> = Pin<Box<dyn Future<Output = Result<()>> + Send + 'a>>;

fn consume(state: AppState, consumer: &PullConsumer) -> BoxedConsumer {
    Box::pin(async move {
        let mut msgs = consumer.messages().await.map_err(natserr)?;
        let state = state.clone(); //ok, cuz it's a bunch of arcs
        tracing::debug!("waiting for messages...");
        while let Some(Ok(msg)) = msgs.next().await{
            tracing::info!("received event ");
            if let Some(req_id) = msg.subject.split(".").last() {
                tracing::debug!("request id: {}", &req_id);
                let res: Vec<u8> = handler(state.clone(), Some(req_id)).await?;
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
            msg.ack().await.map_err(natserr)?;
        } 
        Ok(())
    })
}

pub async fn start_function() -> Result<()> {
    let state = AppState::new().await?;
    let js = &*state.js;

    let consumer_name = format!("{}-{}", settings.project, settings.name);
    let subject = format!("{}.{}.exec.rs.*", settings.project, settings.name);

    tracing::info!("starting consumer listening to subject: {}", &subject);
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
    consume(state, &consumer).await?;
    Ok(())
}
