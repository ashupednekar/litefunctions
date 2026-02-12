package pkg

func StreamHandler(input <-chan []byte) <-chan []byte {
	out := make(chan []byte)
	go func() {
		defer close(out)
		for range input {
			// Default implementation returns no responses.
		}
	}()
	return out
}
