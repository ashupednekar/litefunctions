use crate::{pkg::state::AppState, prelude::Result};

pub async fn handler(state: AppState, req_id: Option<&str>) -> Result<Vec<u8>> {
    Ok("".into())
}
