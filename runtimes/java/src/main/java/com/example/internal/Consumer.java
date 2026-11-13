package com.example.internal;

import java.util.concurrent.ExecutorService;
import java.util.concurrent.Executors;
import java.util.stream.Stream;

import org.slf4j.Logger;
import org.slf4j.LoggerFactory;

import io.nats.client.Dispatcher;

/**
 * NATS consumer mirroring the Go runtime's {@code Consume}/{@code StartFunction}.
 * Requests arrive on {@code <project>.<name>.exec.java.<reqId>} and responses
 * are published to {@code <project>.<name>.res.java.<reqId>}.
 */
public class Consumer implements AutoCloseable {

	private static final Logger log = LoggerFactory.getLogger(Consumer.class);

	private final Settings settings;
	private final Function function;
	private final AppState state;
	private final ExecutorService executor = Executors.newCachedThreadPool();

	private Dispatcher dispatcher;
	private String subject;

	public Consumer(Settings settings, Function function, AppState state) {
		this.settings = settings;
		this.function = function;
		this.state = state;
	}

	public void start() throws Exception {
		String subject = "%s.%s.exec.java.*".formatted(settings.getProject(), settings.getName());
		this.subject = subject;
		String name = "%s-%s".formatted(settings.getProject(), settings.getName());
		log.info("starting consumer subject={} name={}", subject, name);

		dispatcher = state.getNats().createDispatcher(message -> {
			String messageSubject = message.getSubject();
			log.info("received event subject={}", messageSubject);

			String reqId = messageSubject.substring(messageSubject.lastIndexOf('.') + 1);
			log.info("request id extracted request_id={}", reqId);

			byte[] payload = message.getData() == null ? new byte[0] : message.getData();
			executor.submit(() -> handleMessage(reqId, payload));
		});
		dispatcher.subscribe(subject);
		log.info("waiting for messages");
	}

	private void handleMessage(String reqId, byte[] payload) {
		Stream<byte[]> out;
		try {
			out = function.streamHandler(Stream.of(payload));
		}
		catch (Exception e) {
			log.error("stream handler failed request_id={}", reqId, e);
			return;
		}
		if (out == null) {
			log.error("stream handler returned null stream request_id={}", reqId);
			return;
		}

		String responseSubject = "%s.%s.res.java.%s"
				.formatted(settings.getProject(), settings.getName(), reqId);
		try {
			out.forEach(response -> {
				try {
					state.getNats().publish(responseSubject, response == null ? new byte[0] : response);
				}
				catch (Exception e) {
					log.error("failed to publish response request_id={}", reqId, e);
				}
			});
		}
		catch (Exception e) {
			log.error("failed to consume stream request_id={}", reqId, e);
		}
		finally {
			out.close();
		}
	}

	@Override
	public void close() {
		if (dispatcher != null) {
			try {
				if (subject != null) {
					dispatcher.unsubscribe(subject);
				}
			}
			catch (Exception e) {
				log.warn("failed to close nats dispatcher", e);
			}
		}
		executor.shutdownNow();
	}

}
