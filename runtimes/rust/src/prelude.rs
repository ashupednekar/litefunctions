use standard_error::StandardError;

pub type Result<T> = core::result::Result<T, standard_error::StandardError>;

pub fn natserr<E: ToString>(err: E) -> StandardError {
    tracing::error!("err: {}", err.to_string());
    StandardError::new("NATS_ERROR")
}
