use crate::prelude::Result;

#[derive(Debug, Clone)]
pub struct AppState {
}

impl AppState {
    pub async fn new() -> Result<AppState> {
        Ok(AppState {
        })
    }
}
