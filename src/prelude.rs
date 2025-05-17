use standard_error::StandardError;

pub type Result<T> = core::result::Result<T, StandardError>;
