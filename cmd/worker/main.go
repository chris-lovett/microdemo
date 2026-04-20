package main

import (
	"encoding/json"
	"log"
	"net/http"
	"os"
	"time"
)

type resp struct {
	Service   string            `json:"service"`
	Time      string            `json:"time"`
	Hostname  string            `json:"hostname"`
	Env       map[string]string `json:"env,omitempty"`
	RequestID string            `json:"request_id,omitempty"`
	Sum       int               `json:"sum"`
}

func main() {
	mux := http.NewServeMux()

	mux.HandleFunc("/healthz", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("ok"))
	})

	mux.HandleFunc("/headers", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("content-type", "application/json")
		json.NewEncoder(w).Encode(r.Header)
	})

	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		host, _ := os.Hostname()
		out := resp{
			Service:   "worker",
			Time:      time.Now().Format(time.RFC3339Nano),
			Hostname:  host,
			RequestID: r.Header.Get("X-Request-Id"),
			Sum:       2 + 3,
		}
		w.Header().Set("content-type", "application/json")
		json.NewEncoder(w).Encode(out)
	})

	addr := ":8080"
	log.Printf("worker listening on %s", addr)
	log.Fatal(http.ListenAndServe(addr, logRequests(mux)))
}

func logRequests(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		log.Printf("%s %s from=%s ua=%q", r.Method, r.URL.Path, r.RemoteAddr, r.UserAgent())
		next.ServeHTTP(w, r)
	})
}
