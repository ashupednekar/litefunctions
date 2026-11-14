package com.example.internal;

/**
 * Runtime configuration sourced from the environment. Mirrors the Go runtime's
 * {@code Settings}. Values are loaded once and reused.
 */
public final class Settings {

	private static volatile Settings instance;

	private final String project;
	private final String name;

	private final String databaseUrl;
	private final String redisUrl;
	private final String redisPassword;
	private final String natsUrl;

	private final String otlpHost;
	private final String otlpPort;
	private final boolean useTelemetry;

	private final String httpPort;

	private Settings() {
		this.project = env("PROJECT");
		this.name = env("NAME");

		this.databaseUrl = env("DATABASE_URL");
		this.redisUrl = env("REDIS_URL");
		this.redisPassword = env("REDIS_PASSWORD");
		this.natsUrl = env("NATS_URL");

		this.otlpHost = env("OTLP_HOST");
		this.otlpPort = env("OTLP_PORT");
		this.useTelemetry = Boolean.parseBoolean(env("USE_TELEMETRY"));

		this.httpPort = env("HTTP_PORT");
	}

	public static Settings load() {
		Settings local = instance;
		if (local == null) {
			synchronized (Settings.class) {
				local = instance;
				if (local == null) {
					local = new Settings();
					instance = local;
				}
			}
		}
		return local;
	}

	private static String env(String key) {
		String value = System.getenv(key);
		return value == null ? "" : value;
	}

	/**
	 * True when at least one runtime variable is present. Tests run without a
	 * runtime environment and should not attempt to connect to infrastructure.
	 */
	public boolean hasRuntimeConfig() {
		return !project.isBlank() || !name.isBlank() || !databaseUrl.isBlank() || !redisUrl.isBlank()
				|| !natsUrl.isBlank();
	}

	public String getProject() {
		return project;
	}

	public String getName() {
		return name;
	}

	public String getDatabaseUrl() {
		return databaseUrl;
	}

	public String getRedisUrl() {
		return redisUrl;
	}

	public String getRedisPassword() {
		return redisPassword;
	}

	public String getNatsUrl() {
		return natsUrl;
	}

	public String getOtlpHost() {
		return otlpHost;
	}

	public String getOtlpPort() {
		return otlpPort;
	}

	public boolean isUseTelemetry() {
		return useTelemetry;
	}

	public String getHttpPort() {
		return httpPort;
	}

}
