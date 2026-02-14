use crate::pkg::conf::load_settings;
use crate::pkg::function_async::stream_handler;
use crate::pkg::state::AppState;
use crate::pkg::Result;
use futures::StreamExt;

pub async fn start_function(state: AppState) -> Result<()> {
    let settings = load_settings();
    let subject = format!("{}.{}.exec.go.*", settings.project, settings.name);
    tracing::info!(subject = %subject, "starting consumer");

    let mut sub = state.nc.subscribe(subject).await?;
    tracing::info!("waiting for messages");

    while let Some(msg) = sub.next().await {
        let req_id = msg.subject.split('.').last().unwrap_or("");
        if req_id.is_empty() {
            continue;
        }

        tracing::info!(subject = %msg.subject, request_id = %req_id, "received event");

        let (tx, rx) = tokio::sync::mpsc::channel(1);
        let _ = tx.send(msg.payload.to_vec()).await;
        drop(tx);

        let mut out = stream_handler(rx);
        while let Some(res) = out.recv().await {
            state
                .nc
                .publish(
                    format!("{}.{}.res.go.{}", settings.project, settings.name, req_id),
                    res.into(),
                )
                .await?;
        }
    }

    Ok(())
}
