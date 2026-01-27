package server

import (
	"fmt"
	"net/http"
)

func HandleHooks(w http.ResponseWriter, r *http.Request) {
	fmt.Printf("Got Hook")
}
