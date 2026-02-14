use std::env;
use std::sync::OnceLock;

#[derive(Debug, Clone)]
pub struct Settings {
    pub project: String,
    pub name: String,

    pub database_url: String,
    pub redis_url: String,
    pub redis_password: String,
    pub nats_url: String,

    pub otlp_host: String,
    pub otlp_port: String,
    pub use_telemetry: bool,

    pub http_port: String,
}

static SETTINGS: OnceLock<Settings> = OnceLock::new();

pub fn load_settings() -> &'static Settings {
    SETTINGS.get_or_init(Settings::from_env)
}

impl Settings {
    fn from_env() -> Settings {
        Settings {
            project: env_var("PROJECT"),
            name: env_var("NAME"),

            database_url: env_var("DATABASE_URL"),
            redis_url: env_var("REDIS_URL"),
            redis_password: env_var("REDIS_PASSWORD"),
            nats_url: env_var("NATS_URL"),

            otlp_host: env_var("OTLP_HOST"),
            otlp_port: env_var("OTLP_PORT"),
            use_telemetry: parse_bool(&env_var("USE_TELEMETRY")),

            http_port: env_var("HTTP_PORT"),
        }
    }
}

fn env_var(key: &str) -> String {
    env::var(key).unwrap_or_default()
}

fn parse_bool(value: &str) -> bool {
    matches!(value.trim().to_lowercase().as_str(), "1" | "true" | "yes" | "on")
}
