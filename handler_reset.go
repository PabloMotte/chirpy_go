package main

import (
	"log"
	"net/http"
)

// Reset the /app/ subtree hit counter
func (cfg *apiConfig) resetHandler(w http.ResponseWriter, r *http.Request) {
	if cfg.platform == "dev" {
		// delete all users, and all data cascading from users
		if err := cfg.dbQueries.DeleteAllUsers(r.Context()); err != nil {
			log.Printf("Error deleting users: %s", err)
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
		cfg.fileserverHits.Store(0)
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("Hits reset to 0"))
		log.Println("app reset")
		return
	} else {
		w.WriteHeader(http.StatusForbidden)
		return
	}
}
