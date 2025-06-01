use async_nats::jetstream;

use super::{conf::settings, state::AppState};






async fn start_function(state: AppState){
    let js = &*state.js; 
    let consumer_name = format!("{}-{}", &settings.project, &settings.environment);
    let consumer = js.get_or_create_consumer(&consumer_name, jetstream::consumer::Config{
        durable_name: Some(consumer_name.clone()),
        filter_subject: format!("{}.{}.exec.{}", &settings.project, &settings.environment, "".to_string()),
        ..Default::default()
    });
}
