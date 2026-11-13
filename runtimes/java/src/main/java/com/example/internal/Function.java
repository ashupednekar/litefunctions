package com.example.internal;

import java.util.stream.Stream;

/**
 * Runtime function contract. The generated {@code FunctionImpl} (or the
 * default implementation shipped with the runtime) fulfils this interface.
 */
public interface Function {

	/** Handle a single HTTP request body. */
	byte[] handle(byte[] body);

	/** Handle a stream of NATS payloads and emit zero or more responses. */
	Stream<byte[]> streamHandler(Stream<byte[]> input);

}
