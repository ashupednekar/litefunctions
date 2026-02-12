package pkg

import (
	"encoding/json"
	"math/rand"
)

func randomWord() string {
	words := []string{"apple", "banana", "cherry", "date", "elderberry"}
	return words[rand.Intn(len(words))]
}

type Payload struct {
	Word string `json:"word"`
}

func StreamHandler(input <-chan []byte) <-chan []byte {
	out := make(chan []byte)

	go func() {
		defer close(out)

		for range input {
			data := Payload{
				Word: randomWord(),
			}

			jsonBytes, err := json.Marshal(data)
			if err != nil {
				continue // safer than panic in a stream
			}

			out <- jsonBytes
		}
	}()

	return out
}
