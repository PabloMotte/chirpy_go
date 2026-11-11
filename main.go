package main

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"sync/atomic"
	"time"

	"github.com/joho/godotenv"
	_ "github.com/lib/pq"
	"github.com/pablomotte/chirpy_go/internal/database"
)

type apiConfig struct {
	fileserverHits atomic.Int32
	dbQueries      *database.Queries
	platform       string
}

type chirpyResponseError struct {
	Error string `json:"error"`
}

// The chirpy_go web server
func main() {
	godotenv.Load()
	dbURL := os.Getenv("DB_URL")
	db, err := sql.Open("postgres", dbURL)
	if err != nil {
		log.Fatalf("Database failed to open: %v", err)
	}
	appPath := "."
	apiCfg := apiConfig{
		fileserverHits: atomic.Int32{},
		dbQueries:      database.New(db),
		platform:       os.Getenv("PLATFORM"),
	}
	chirpy_go := http.NewServeMux()

	// the /app/ subtree...
	chirpy_go.Handle("/app/", apiCfg.middlewareMetricsInc(http.StripPrefix("/app", http.FileServer(http.Dir(appPath)))))

	// the /api/ subtree...
	// the GET /api/healthz endpoint displays the "health" of the server
	chirpy_go.HandleFunc("GET /api/healthz", healthzHandler)
	// the POST /api/chirps endpoint creates a chirp
	chirpy_go.HandleFunc("POST /api/chirps", apiCfg.createChirpHandler)
	// the GET /api/chirps endpoint gets all chirps
	chirpy_go.HandleFunc("GET /api/chirps", apiCfg.getChirpsHandler)
	// the GET /api/chirps/{chirpID} endpoint gets a single chirp with UUID chirpID
	chirpy_go.HandleFunc("GET /api/chirps/{chirpID}", apiCfg.getChirpHandler)
	// the POST /api/users endpoint creates a user
	chirpy_go.HandleFunc("POST /api/users", apiCfg.createUserHandler)
	// the POST /api/login endpoint logs in a user
	chirpy_go.HandleFunc("POST /api/login", apiCfg.loginUserHandler)

	// the /admin/ subtree...
	// the /admin/metrics endpoint displays the number of requests handled under the /app/ subtree
	chirpy_go.HandleFunc("GET /admin/metrics", apiCfg.metricsHandler)
	// the /admin/reset endpoint resets the /app/ metrics
	chirpy_go.HandleFunc("POST /admin/reset", apiCfg.resetHandler)

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

// Increment the /app/ metrics counter
func (cfg *apiConfig) middlewareMetricsInc(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		cfg.fileserverHits.Add(1)
		next.ServeHTTP(w, r)
	})
}

// Display the number of hits to the /app/ subtree
func (cfg *apiConfig) metricsHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.WriteHeader(http.StatusOK)
	s := fmt.Sprintf(`<html>
  <body>
    <h1>Welcome, Chirpy Admin</h1>
    <p>Chirpy has been visited %d times!</p>
  </body>
</html>`, cfg.fileserverHits.Load())
	w.Write(([]byte)(s))
}

func (cfg *apiConfig) responseWithJSON(respId int, respBody interface{}, w http.ResponseWriter, r *http.Request) {
	if respId >= http.StatusInternalServerError {
		log.Printf("Responding with %d error: %v", respId, respBody)
	}
	dat, err := json.Marshal(respBody)
	if err != nil {
		log.Printf("Error marshalling JSON: %s", err)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(respId)
	w.Write(dat)
}
