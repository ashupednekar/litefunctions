package com.example.internal.fn;

import java.nio.charset.StandardCharsets;
import java.util.concurrent.ThreadLocalRandom;
import java.util.stream.Stream;

class Handler {

	private static final String[] WORDS = { "apple", "banana", "cherry", "date", "elderberry" };

	public Stream<byte[]> streamHandler(Stream<byte[]> input) {
		return input.map(ignored -> {
			String word = WORDS[ThreadLocalRandom.current().nextInt(WORDS.length)];
			return ("{\"word\":\"" + word + "\"}").getBytes(StandardCharsets.UTF_8);
		});
	}

}
