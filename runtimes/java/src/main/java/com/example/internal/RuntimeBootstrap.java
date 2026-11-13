package com.example.internal;

import org.slf4j.Logger;
import org.slf4j.LoggerFactory;
import org.springframework.boot.context.event.ApplicationReadyEvent;
import org.springframework.context.event.EventListener;
import org.springframework.stereotype.Component;

import jakarta.annotation.PreDestroy;

/**
 * Starts the NATS consumer once the Spring context is ready and closes runtime
 * resources on shutdown.
 */
@Component
public class RuntimeBootstrap {

	private static final Logger log = LoggerFactory.getLogger(RuntimeBootstrap.class);

	private final Function function;

	private AppState state;
	private Consumer consumer;

	public RuntimeBootstrap(Function function) {
		this.function = function;
	}

	@EventListener(ApplicationReadyEvent.class)
	public void start() throws Exception {
		Settings settings = Settings.load();
		if (!settings.hasRuntimeConfig()) {
			log.warn("runtime environment is not configured; skipping state initialization");
			return;
		}

		try {
			state = new AppState(settings);
			consumer = new Consumer(settings, function, state);
			consumer.start();
			log.info("java runtime started project={} function={}", settings.getProject(), settings.getName());
		}
		catch (Exception e) {
			log.error("failed to initialize runtime state", e);
			close();
			throw e;
		}
	}

	@PreDestroy
	public void close() {
		if (consumer != null) {
			consumer.close();
			consumer = null;
		}
		if (state != null) {
			state.close();
			state = null;
		}
	}

}
