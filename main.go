package main

import (
	"fmt"
	"log"
	"net/http"
	"sync/atomic"
	"time"
)

type apiConfig struct {
	fileserverHits atomic.Int32
}

// The chirpy_go web server
func main() {
	appPath := "."
	apiCfg := apiConfig{
		fileserverHits: atomic.Int32{},
	}
	chirpy_go := http.NewServeMux()
	// the /app/ subtree...
	chirpy_go.Handle("/app/", apiCfg.middlewareMetricsInc(http.StripPrefix("/app", http.FileServer(http.Dir(appPath)))))
	// the /healtz endpoint displays the "health" of the server
	chirpy_go.HandleFunc("GET /healthz", healthzHandler)
	// the /metrics endpoint displays the number of requests handled under the /app/ subtree
	chirpy_go.HandleFunc("GET /metrics", apiCfg.metricsHandler)
	// the /reset endpoint resets the /app/ metrics
	chirpy_go.HandleFunc("POST /reset", apiCfg.resetHandler)
	s := &http.Server{
		Addr:           ":8080",
		Handler:        apiCfg.middlewareLog(chirpy_go),
		ReadTimeout:    10 * time.Second,
		WriteTimeout:   10 * time.Second,
		MaxHeaderBytes: 1 << 20,
	}
	fmt.Printf("Starting chirpy_go server on %s\n", s.Addr)
	log.Fatal(s.ListenAndServe())
}

// Log all HTTP traffic
func (cfg *apiConfig) middlewareLog(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		log.Printf("%s %s", r.Method, r.URL.Path)
		next.ServeHTTP(w, r)
	})
}

// Display the number of hits to the /app/ subtree
func (cfg *apiConfig) metricsHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	w.WriteHeader(http.StatusOK)
	s := fmt.Sprintf("Hits: %v", cfg.fileserverHits.Load())
	w.Write(([]byte)(s))
}

// Increment the /app/ metrics counter
func (cfg *apiConfig) middlewareMetricsInc(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		cfg.fileserverHits.Add(1)
		next.ServeHTTP(w, r)
	})
}
