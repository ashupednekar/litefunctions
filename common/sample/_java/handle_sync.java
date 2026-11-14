package com.example.internal.fn;

import com.example.internal.fn.types.Input;
import com.example.internal.fn.types.Output;
import com.fasterxml.jackson.databind.ObjectMapper;

class Handler {

	private static final ObjectMapper MAPPER = new ObjectMapper();

	public byte[] handle(byte[] body) {
		try {
			Input input = body == null || body.length == 0
					? new Input()
					: MAPPER.readValue(body, Input.class);

			String name = input.name == null || input.name.isBlank() ? "stranger" : input.name;
			return MAPPER.writeValueAsBytes(new Output("Hello " + name + " from Java!"));
		}
		catch (Exception e) {
			throw new RuntimeException("invalid request", e);
		}
	}

}
