package main

import (
	"fmt"
	"log"
	"net/http"
	"time"
)

// The chirpy_go web server
func main() {
	appPath := "."
	chirpy_go := http.NewServeMux()
	chirpy_go.Handle("/app/", http.StripPrefix("/app", http.FileServer(http.Dir(appPath))))
	chirpy_go.HandleFunc("/healthz", healthzHandler)
	s := &http.Server{
		Addr:           ":8080",
		Handler:        chirpy_go,
		ReadTimeout:    10 * time.Second,
		WriteTimeout:   10 * time.Second,
		MaxHeaderBytes: 1 << 20,
	}
	fmt.Printf("Starting chirpy_go server on %s\n", s.Addr)
	log.Fatal(s.ListenAndServe())
}

func healthzHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	w.WriteHeader(200)
	w.Write(([]byte)("OK"))
}
