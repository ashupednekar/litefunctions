package com.example.internal.fn;

import java.nio.charset.StandardCharsets;
import java.util.concurrent.ThreadLocalRandom;
import java.util.stream.Stream;

import org.springframework.stereotype.Component;

import com.example.internal.Function;

/**
 * Default function implementation. The static build replaces this file with a
 * generated wrapper that delegates to the user's {@code handle.java}.
 *
 * <p>This default mirrors the Go runtime's stream handler: every input payload
 * produces a JSON object containing a random word.</p>
 */
@Component
public class FunctionImpl implements Function {

	private static final String[] WORDS = { "apple", "banana", "cherry", "date", "elderberry" };

	@Override
	public byte[] handle(byte[] body) {
		return new byte[0];
	}

	@Override
	public Stream<byte[]> streamHandler(Stream<byte[]> input) {
		return input.map(ignored -> randomWordPayload());
	}

	private static byte[] randomWordPayload() {
		String word = WORDS[ThreadLocalRandom.current().nextInt(WORDS.length)];
		return ("{\"word\":\"" + word + "\"}").getBytes(StandardCharsets.UTF_8);
	}

}
