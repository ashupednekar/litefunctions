package com.example.internal;

import static org.springframework.test.web.servlet.request.MockMvcRequestBuilders.request;
import static org.springframework.test.web.servlet.result.MockMvcResultMatchers.content;
import static org.springframework.test.web.servlet.result.MockMvcResultMatchers.status;

import java.util.List;

import org.junit.jupiter.api.Test;
import org.springframework.http.HttpMethod;
import org.springframework.test.web.servlet.MockMvc;
import org.springframework.test.web.servlet.setup.MockMvcBuilders;

/**
 * Verifies the HTTP entrypoint mirrors the Go runtime: every method and every
 * path (including nested paths) is routed to {@link Function#handle}.
 */
class HandlerRoutingTests {

	private final MockMvc mockMvc = MockMvcBuilders
			.standaloneSetup(new handler(new EchoFunction()))
			.build();

	@Test
	void routesEveryMethodAndPathToTheFunction() throws Exception {
		for (String method : List.of("GET", "POST", "PUT", "PATCH", "DELETE", "OPTIONS", "HEAD")) {
			for (String path : List.of("/", "/nested", "/deep/nested/path")) {
				mockMvc.perform(request(HttpMethod.valueOf(method), path))
						.andExpect(status().isOk());
			}
		}
	}

	@Test
	void passesRequestBodyToTheFunction() throws Exception {
		mockMvc.perform(request(HttpMethod.POST, "/deep/nested/path").content("ping"))
				.andExpect(status().isOk())
				.andExpect(content().bytes("ping".getBytes()));
	}

	private static final class EchoFunction implements Function {

		@Override
		public byte[] handle(byte[] body) {
			return body;
		}

		@Override
		public java.util.stream.Stream<byte[]> streamHandler(java.util.stream.Stream<byte[]> input) {
			return input;
		}

	}

}
