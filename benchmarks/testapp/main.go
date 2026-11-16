package main

import (
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"strconv"
	"time"
)

const maxPayloadSize = 10 << 20

var payloads map[int][]byte

func main() {
	payloads = make(map[int][]byte)
	for _, size := range []int{1024, 16 << 10, 256 << 10, 1 << 20, 10 << 20} {
		p := make([]byte, size)
		for i := range p {
			p[i] = byte((i * 31) % 251)
		}
		payloads[size] = p
	}

	mux := http.NewServeMux()
	mux.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		fmt.Println("Responding")
		w.Header().Set("Content-Type", "text/plain")
		_, _ = io.WriteString(w, "ok")
	})
	mux.HandleFunc("/payload", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		size, err := strconv.Atoi(r.URL.Query().Get("size"))
		if err != nil {
			http.Error(w, "invalid size", http.StatusBadRequest)
			return
		}
		p, ok := payloads[size]
		if !ok {
			http.Error(w, "unsupported size", http.StatusBadRequest)
			return
		}
		w.Header().Set("Content-Type", "application/octet-stream")
		w.Header().Set("Content-Length", strconv.Itoa(len(p)))
		_, _ = w.Write(p)
	})
	mux.HandleFunc("/echo", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		if r.ContentLength > maxPayloadSize {
			http.Error(w, "payload too large", http.StatusRequestEntityTooLarge)
			return
		}
		defer r.Body.Close()
		body, err := io.ReadAll(io.LimitReader(r.Body, maxPayloadSize+1))
		if err != nil {
			http.Error(w, "failed to read request body", http.StatusBadRequest)
			return
		}
		if len(body) > maxPayloadSize {
			http.Error(w, "payload too large", http.StatusRequestEntityTooLarge)
			return
		}
		w.Header().Set("Content-Type", "application/octet-stream")
		_, _ = w.Write(body)
	})

	srv := &http.Server{
		Addr:              envOr("ASHRIX_BENCH_ADDR", "127.0.0.1:8081"),
		Handler:           mux,
		ReadHeaderTimeout: 5 * time.Second,
		IdleTimeout:       60 * time.Second,
	}
	log.Printf("benchmark app listening on %s", srv.Addr)
	log.Fatal(srv.ListenAndServe())
}

func envOr(key, fallback string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return fallback
}
