package com.example.internal;

import java.net.URI;
import java.net.URLDecoder;
import java.nio.charset.StandardCharsets;
import java.util.Locale;

import javax.sql.DataSource;

import org.slf4j.Logger;
import org.slf4j.LoggerFactory;
import org.springframework.data.redis.connection.RedisConnection;
import org.springframework.data.redis.connection.RedisPassword;
import org.springframework.data.redis.connection.RedisStandaloneConfiguration;
import org.springframework.data.redis.connection.lettuce.LettuceConnectionFactory;

import com.zaxxer.hikari.HikariConfig;
import com.zaxxer.hikari.HikariDataSource;

import io.nats.client.Connection;
import io.nats.client.Nats;

/**
 * Shared runtime state: Postgres pool, Redis connection factory and NATS
 * connection. Mirrors the Go runtime's {@code AppState}.
 */
public class AppState implements AutoCloseable {

	private static final Logger log = LoggerFactory.getLogger(AppState.class);

	private final HikariDataSource dataSource;
	private final LettuceConnectionFactory redisFactory;
	private final Connection nats;

	public AppState(Settings settings) throws Exception {
		validateSettings(settings);

		log.info("settings: project={} name={} nats_url={} database_url_set={} redis_url_set={}",
				settings.getProject(), settings.getName(), safeUrlSummary(settings.getNatsUrl()),
				!settings.getDatabaseUrl().isBlank(), !settings.getRedisUrl().isBlank());

		this.dataSource = buildDataSource(settings);
		this.redisFactory = buildRedisFactory(settings);
		try (RedisConnection connection = redisFactory.getConnection()) {
			connection.ping();
			connection.ping();
		}

		this.nats = Nats.connect(settings.getNatsUrl());
	}

	private static HikariDataSource buildDataSource(Settings settings) {
		HikariConfig config = new HikariConfig();

		String raw = settings.getDatabaseUrl();
		if (raw.startsWith("jdbc:")) {
			config.setJdbcUrl(raw);
		}
		else {
			UriParts db = parseUri(raw);
			StringBuilder jdbcUrl = new StringBuilder("jdbc:postgresql://")
					.append(db.host.isEmpty() ? "localhost" : db.host);
			if (db.port > 0) {
				jdbcUrl.append(':').append(db.port);
			}
			jdbcUrl.append(db.path.isEmpty() ? "/" : db.path);
			if (!db.query.isEmpty()) {
				jdbcUrl.append('?').append(db.query);
			}
			config.setJdbcUrl(jdbcUrl.toString());
			if (!db.username.isEmpty()) {
				config.setUsername(db.username);
			}
			if (!db.password.isEmpty()) {
				config.setPassword(db.password);
			}
		}

		config.setMaximumPoolSize(5);
		config.setInitializationFailTimeout(-1);
		return new HikariDataSource(config);
	}

	private static LettuceConnectionFactory buildRedisFactory(Settings settings) {
		UriParts redis = parseUri(settings.getRedisUrl());

		RedisStandaloneConfiguration config = new RedisStandaloneConfiguration();
		config.setHostName(redis.host.isEmpty() ? "localhost" : redis.host);
		config.setPort(redis.port > 0 ? redis.port : 6379);
		if (!redis.username.isEmpty()) {
			config.setUsername(redis.username);
		}
		if (!redis.password.isEmpty()) {
			config.setPassword(RedisPassword.of(redis.password));
		}
		else if (!settings.getRedisPassword().isBlank()) {
			config.setPassword(RedisPassword.of(settings.getRedisPassword()));
		}
		if (redis.database > 0) {
			config.setDatabase(redis.database);
		}

		LettuceConnectionFactory factory = new LettuceConnectionFactory(config);
		factory.afterPropertiesSet();
		return factory;
	}

	private static void validateSettings(Settings settings) {
		if (settings.getProject().isBlank()) {
			throw new IllegalStateException("ERR-SETTINGS: PROJECT is required (set env PROJECT)");
		}
		if (settings.getName().isBlank()) {
			throw new IllegalStateException("ERR-SETTINGS: NAME is required (set env NAME)");
		}
		if (settings.getDatabaseUrl().isBlank()) {
			throw new IllegalStateException("ERR-SETTINGS: DATABASE_URL is required (set env DATABASE_URL)");
		}
		if (settings.getRedisUrl().isBlank()) {
			throw new IllegalStateException("ERR-SETTINGS: REDIS_URL is required (set env REDIS_URL)");
		}
		if (settings.getNatsUrl().isBlank()) {
			throw new IllegalStateException("ERR-SETTINGS: NATS_URL is required (set env NATS_URL)");
		}
	}

	private static String safeUrlSummary(String raw) {
		if (raw == null || raw.isBlank()) {
			return "(empty)";
		}
		try {
			URI uri = URI.create(raw);
			if (uri.getScheme() == null && uri.getHost() == null) {
				return "(invalid)";
			}
			return uri.getScheme() + "://" + uri.getHost();
		}
		catch (IllegalArgumentException e) {
			return "(invalid)";
		}
	}

	private static UriParts parseUri(String raw) {
		UriParts parts = new UriParts();
		if (raw == null || raw.isBlank()) {
			return parts;
		}

		String normalized = raw;
		String lower = raw.toLowerCase(Locale.ROOT);
		if (lower.startsWith("jdbc:")) {
			normalized = raw.substring("jdbc:".length());
		}
		if (!lower.startsWith("postgres://") && !lower.startsWith("postgresql://")
				&& !lower.startsWith("redis://") && !lower.startsWith("rediss://")) {
			// Key/value DSN or host:port form. Let the driver handle it.
			parts.path = normalized;
			return parts;
		}

		URI uri = URI.create(normalized);
		parts.host = uri.getHost() == null ? "" : uri.getHost();
		parts.port = uri.getPort();
		parts.path = uri.getPath() == null ? "" : uri.getPath();
		parts.query = uri.getQuery() == null ? "" : uri.getQuery();

		String userInfo = uri.getUserInfo();
		if (userInfo != null && !userInfo.isEmpty()) {
			String[] credentials = userInfo.split(":", 2);
			if (credentials.length == 2) {
				parts.username = decode(credentials[0]);
				parts.password = decode(credentials[1]);
			}
			else {
				parts.username = decode(credentials[0]);
			}
		}

		if (!parts.path.isEmpty() && parts.path.length() > 1) {
			String maybeDb = parts.path.substring(1);
			try {
				parts.database = Integer.parseInt(maybeDb);
				if (uri.getScheme() != null && uri.getScheme().startsWith("redis")) {
					parts.path = "/" + parts.database;
				}
			}
			catch (NumberFormatException ignored) {
				// path is a database name, not a Redis database index
			}
		}

		return parts;
	}

	private static String decode(String value) {
		return URLDecoder.decode(value, StandardCharsets.UTF_8);
	}

	public DataSource getDataSource() {
		return dataSource;
	}

	public LettuceConnectionFactory getRedisFactory() {
		return redisFactory;
	}

	public Connection getNats() {
		return nats;
	}

	@Override
	public void close() {
		try {
			if (nats != null) {
				nats.close();
			}
		}
		catch (Exception e) {
			log.warn("failed to close nats connection", e);
		}
		try {
			if (redisFactory != null) {
				redisFactory.destroy();
			}
		}
		catch (Exception e) {
			log.warn("failed to close redis connection factory", e);
		}
		try {
			if (dataSource != null) {
				dataSource.close();
			}
		}
		catch (Exception e) {
			log.warn("failed to close database pool", e);
		}
	}

	private static final class UriParts {

		private String host = "";
		private int port = -1;
		private String path = "";
		private String query = "";
		private String username = "";
		private String password = "";
		private int database = -1;

	}

}
