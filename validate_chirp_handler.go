package main

import (
	"encoding/json"
	"log"
	"net/http"
	"strings"
)

func (cfg *apiConfig) validateChirpHandler(w http.ResponseWriter, r *http.Request) {

	decoder := json.NewDecoder(r.Body)
	chirpReq := chirpRequest{}
	err := decoder.Decode(&chirpReq)
	if err != nil {
		log.Printf("Error decoding parameters: %s", err)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}
	if len(chirpReq.Body) <= 0 {
		respBody := chirpResponseError{
			Error: "Chirp is empty",
		}
		cfg.responseWithJSON(http.StatusBadRequest, respBody, w, r)
	} else if len(chirpReq.Body) > 140 {
		respBody := chirpResponseError{
			Error: "Chirp is too long",
		}
		cfg.responseWithJSON(http.StatusBadRequest, respBody, w, r)
	} else {
		respBody := chirpResponseValid{
			CleanedBody: profanity_check(chirpReq.Body),
		}
		cfg.responseWithJSON(http.StatusOK, respBody, w, r)
	}
}

func profanity_check(s string) string {
	explicit_words := []string{
		"kerfuffle",
		"sharbert",
		"fornax",
	}
	words := strings.Split(s, " ")
	for k, v := range words {
		for _, j := range explicit_words {
			if strings.EqualFold(v, j) {
				words[k] = "****"
			}
		}
	}
	return strings.Join(words, " ")
}
