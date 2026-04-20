package main

import (
	"encoding/json"
	"io"
	"log"
	"net/http"
	"os"
	"time"
)

type combined struct {
	Service  string          `json:"service"`
	Time     string          `json:"time"`
	Hostname string          `json:"hostname"`
	APIURL   string          `json:"api_url"`
	API      json.RawMessage `json:"api"`
}

func main() {
	apiURL := getenv("API_URL", "http://api:8080/")

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

		res, err := http.Get(apiURL)
		if err != nil {
			http.Error(w, "api call failed: "+err.Error(), http.StatusBadGateway)
			return
		}
		defer res.Body.Close()

		b, _ := io.ReadAll(res.Body)
		if res.StatusCode/100 != 2 {
			http.Error(w, "api returned non-2xx: "+string(b), http.StatusBadGateway)
			return
		}

		out := combined{
			Service:  "frontend",
			Time:     time.Now().Format(time.RFC3339Nano),
			Hostname: host,
			APIURL:   apiURL,
			API:      json.RawMessage(b),
		}
		w.Header().Set("content-type", "application/json")
		json.NewEncoder(w).Encode(out)
	})

	addr := ":8080"
	log.Printf("frontend listening on %s; API_URL=%s", addr, apiURL)
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
