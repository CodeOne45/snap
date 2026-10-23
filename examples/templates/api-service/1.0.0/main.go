package main

import (
	"fmt"
	"log"
	"net/http"
)

func main() {
	http.HandleFunc("/health", func(w http.ResponseWriter, _ *http.Request) {
		fmt.Fprintln(w, "{{name}} is healthy")
	})
	addr := ":{{port}}"
	log.Printf("{{name}} listening on %s", addr)
	log.Fatal(http.ListenAndServe(addr, nil))
}
