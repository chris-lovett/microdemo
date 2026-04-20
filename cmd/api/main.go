package main

import (
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"time"
)

type combined struct {
	Service   string          `json:"service"`
	Time      string          `json:"time"`
	Hostname  string          `json:"hostname"`
	WorkerURL string          `json:"worker_url"`
	Worker    json.RawMessage `json:"worker"`
}

func main() {
	workerURL := getenv("WORKER_URL", "http://worker:8080/")

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

		req, _ := http.NewRequestWithContext(r.Context(), http.MethodGet, workerURL, nil)
		// propagate a request id if present; generate a simple one otherwise
		rid := r.Header.Get("X-Request-Id")
		if rid == "" {
			rid = fmt.Sprintf("rid-%d", time.Now().UnixNano())
		}
		req.Header.Set("X-Request-Id", rid)

		res, err := http.DefaultClient.Do(req)
		if err != nil {
			http.Error(w, "worker call failed: "+err.Error(), http.StatusBadGateway)
			return
		}
		defer res.Body.Close()

		b, _ := io.ReadAll(res.Body)
		if res.StatusCode/100 != 2 {
			http.Error(w, "worker returned non-2xx: "+string(b), http.StatusBadGateway)
			return
		}

		out := combined{
			Service:   "api",
			Time:      time.Now().Format(time.RFC3339Nano),
			Hostname:  host,
			WorkerURL: workerURL,
			Worker:    json.RawMessage(b),
		}
		w.Header().Set("content-type", "application/json")
		w.Header().Set("X-Request-Id", rid)
		json.NewEncoder(w).Encode(out)
	})

	addr := ":8080"
	log.Printf("api listening on %s; WORKER_URL=%s", addr, workerURL)
	log.Fatal(http.ListenAndServe(addr, logRequests(mux)))
}

func getenv(k, def string) string {
	if v := os.Getenv(k); v != "" {
		return v
	}
	return def
}

func logRequests(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		log.Printf("%s %s from=%s ua=%q", r.Method, r.URL.Path, r.RemoteAddr, r.UserAgent())
		next.ServeHTTP(w, r)
	})
}
