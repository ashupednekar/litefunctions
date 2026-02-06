use crate::{prelude::Result, pkg::conf::settings, pkg::state::AppState};
use uuid::Uuid;

pub async fn handler(state: AppState, req_id: Option<&str>, _body: Vec<u8>) -> Result<Vec<u8>>{
    tracing::debug!("handler triggered");
  Ok(Uuid::new_v4().to_string().into())
}
