package com.example.internal;

import org.springframework.http.ResponseEntity;
import org.springframework.web.bind.annotation.RequestBody;
import org.springframework.web.bind.annotation.RequestMapping;
import org.springframework.web.bind.annotation.RestController;

/**
 * HTTP entrypoint. Matches every path and every method, mirroring the Go
 * runtime's {@code http.ServeMux} handler mounted at {@code "/"}.
 */
@RestController
public class handler {

	private final Function function;

	public handler(Function function) {
		this.function = function;
	}

	@RequestMapping({ "/", "/**" })
	public ResponseEntity<byte[]> handle(@RequestBody(required = false) byte[] body) {
		byte[] response = function.handle(body == null ? new byte[0] : body);
		return ResponseEntity.ok().body(response == null ? new byte[0] : response);
	}

}
