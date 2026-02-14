use rand::seq::SliceRandom;
use serde::Serialize;
use tokio::sync::mpsc::{self, Receiver};

#[derive(Serialize)]
struct Payload {
    word: String,
}

fn random_word() -> String {
    let words = ["apple", "banana", "cherry", "date", "elderberry"];
    words
        .choose(&mut rand::thread_rng())
        .unwrap_or(&"apple")
        .to_string()
}

pub fn stream_handler(mut input: Receiver<Vec<u8>>) -> Receiver<Vec<u8>> {
    let (tx, rx) = mpsc::channel(16);

    tokio::spawn(async move {
        while input.recv().await.is_some() {
            let payload = Payload {
                word: random_word(),
            };
            if let Ok(json_bytes) = serde_json::to_vec(&payload) {
                if tx.send(json_bytes).await.is_err() {
                    break;
                }
            }
        }
    });

    rx
}
