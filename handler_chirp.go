package main

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/lib/pq"
	"github.com/lib/pq/pqerror"
	"github.com/pablomotte/chirpy_go/internal/auth"
	"github.com/pablomotte/chirpy_go/internal/database"
)

type createChirpRequest struct {
	Body string `json:"body"`
	// UserID string `json:"user_id"`
}

type chirpResponseValid struct {
	Id        string `json:"id"`
	CreatedAt string `json:"created_at"`
	UpdatedAt string `json:"updated_at"`
	Body      string `json:"body"`
	UserID    string `json:"user_id"`
}

func (cfg *apiConfig) getChirpsHandler(w http.ResponseWriter, r *http.Request) {
	dbChirps, err := cfg.dbQueries.GetAllChirps(r.Context())
	if err != nil {
		if err == sql.ErrNoRows {
			// log.Printf("No chirps available")
			w.WriteHeader(http.StatusNotFound)
			return
		}
		log.Printf("Error retrieving chirps: %s", err)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}
	chirps := []chirpResponseValid{}
	for _, dbChirp := range dbChirps {
		chirps = append(chirps, chirpResponseValid{
			Id:        dbChirp.ID.String(),
			CreatedAt: dbChirp.CreatedAt.String(),
			UpdatedAt: dbChirp.UpdatedAt.String(),
			UserID:    dbChirp.UserID.String(),
			Body:      dbChirp.Body,
		})
	}
	cfg.responseWithJSON(http.StatusOK, chirps, w, r)
}

func (cfg *apiConfig) getChirpHandler(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("chirpID")
	if len(id) <= 0 {
		cfg.getChirpsHandler(w, r)
		return
	}
	valid_id, err_id := uuid.Parse(id)
	if err_id != nil {
		respBody := chirpyResponseError{
			Error: fmt.Sprintf("Chirp ID is an invalid UUID format: %v", id),
		}
		cfg.responseWithJSON(http.StatusBadRequest, respBody, w, r)
		return
	}
	dbChirp, err := cfg.dbQueries.GetChirp(r.Context(), valid_id)
	if err != nil {
		if err == sql.ErrNoRows {
			// log.Printf("No chirps available")
			w.WriteHeader(http.StatusNotFound)
			return
		}
		log.Printf("Error retrieving chirp %v: %s", valid_id, err)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}
	chirp := chirpResponseValid{
		Id:        dbChirp.ID.String(),
		CreatedAt: dbChirp.CreatedAt.String(),
		UpdatedAt: dbChirp.UpdatedAt.String(),
		UserID:    dbChirp.UserID.String(),
		Body:      dbChirp.Body,
	}
	cfg.responseWithJSON(http.StatusOK, chirp, w, r)
}

func (cfg *apiConfig) createChirpHandler(w http.ResponseWriter, r *http.Request) {
	used_token, err := auth.GetBearerToken(r.Header)
	if err != nil {
		respBody := chirpyResponseError{
			Error: fmt.Sprintf("Invalid authentication token: %v", err),
		}
		cfg.responseWithJSON(http.StatusUnauthorized, respBody, w, r)
		return
	}
	passed_uuid, err := auth.ValidateJWT(used_token, cfg.jwt_secret)
	if err != nil || passed_uuid == uuid.Nil {
		log.Printf("Error validating token: %v", used_token)
		log.Printf("Error validating token: %v", passed_uuid)
		respBody := chirpyResponseError{
			Error: fmt.Sprintf("Invalid authentication token: %v", err),
		}
		cfg.responseWithJSON(http.StatusUnauthorized, respBody, w, r)
		return
	}
	decoder := json.NewDecoder(r.Body)
	chirpReq := createChirpRequest{}
	err = decoder.Decode(&chirpReq)
	if err != nil {
		log.Printf("Error decoding parameters: %s", err)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}
	if len(chirpReq.Body) <= 0 {
		respBody := chirpyResponseError{
			Error: "Chirp is empty",
		}
		cfg.responseWithJSON(http.StatusBadRequest, respBody, w, r)
	} else if len(chirpReq.Body) > 140 {
		respBody := chirpyResponseError{
			Error: "Chirp is too long",
		}
		cfg.responseWithJSON(http.StatusBadRequest, respBody, w, r)
		// } else if err := uuid.Validate(chirpReq.UserID); err != nil {
		// 	respBody := chirpyResponseError{
		// 		Error: "Invalid UUID format",
		// 	}
		// 	cfg.responseWithJSON(http.StatusBadRequest, respBody, w, r)
	} else {
		// passed_uuid, err := uuid.Parse(chirpReq.UserID)
		// if err != nil {
		// 	respBody := chirpyResponseError{
		// 		Error: "Unable to parse UUID",
		// 	}
		// 	cfg.responseWithJSON(http.StatusBadRequest, respBody, w, r)
		// 	return
		// }
		chirpCreateParms := database.CreateChirpParams{
			Body:   profanity_check(chirpReq.Body),
			UserID: passed_uuid,
		}
		chirp, err := cfg.dbQueries.CreateChirp(r.Context(), chirpCreateParms)
		if err != nil {
			if pqErr := pq.As(err, pqerror.UniqueViolation); pqErr != nil {
				error_text := fmt.Sprintf("chirp already exists: %v", chirpCreateParms)
				log.Println(error_text)
				respBody := chirpyResponseError{
					Error: error_text,
				}
				cfg.responseWithJSON(http.StatusBadRequest, respBody, w, r)
			} else if pqErr := pq.As(err, pqerror.ForeignKeyViolation); pqErr != nil {
				error_text := fmt.Sprintf("user is invalid: %v", chirpCreateParms.UserID)
				log.Println(error_text)
				respBody := chirpyResponseError{
					Error: error_text,
				}
				cfg.responseWithJSON(http.StatusBadRequest, respBody, w, r)
			} else {
				log.Printf("Error creating chirp: %s", err)
				w.WriteHeader(http.StatusInternalServerError)
			}
			return
		}
		respBody := chirpResponseValid{
			Id:        chirp.ID.String(),
			CreatedAt: chirp.CreatedAt.Format(time.UnixDate),
			UpdatedAt: chirp.UpdatedAt.Format(time.UnixDate),
			Body:      chirp.Body,
			UserID:    chirp.UserID.String(),
		}
		cfg.responseWithJSON(http.StatusCreated, respBody, w, r)
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
