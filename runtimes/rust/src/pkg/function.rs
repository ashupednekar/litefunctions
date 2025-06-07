use crate::{prelude::Result, pkg::state::AppState};


pub async fn handler(state: AppState) -> Result<Vec<u8>>{
    Ok("".into())
}
