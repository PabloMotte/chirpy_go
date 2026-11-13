package main

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"time"

	"github.com/google/uuid"
	"github.com/lib/pq"
	"github.com/lib/pq/pqerror"
	"github.com/pablomotte/chirpy_go/internal/auth"
	"github.com/pablomotte/chirpy_go/internal/database"
)

type userCreateRequest struct {
	Email    string `json:"email"`
	Password string `json:"password"`
}

type userCreateResponseValid struct {
	Id        string `json:"id"`
	CreatedAt string `json:"created_at"`
	UpdatedAt string `json:"updated_at"`
	Email     string `json:"email"`
}

type userLoginRequest struct {
	Email    string `json:"email"`
	Password string `json:"password"`
}

type userLoginResponseValid struct {
	Id           string `json:"id"`
	CreatedAt    string `json:"created_at"`
	UpdatedAt    string `json:"updated_at"`
	Email        string `json:"email"`
	Token        string `json:"token"`
	RefreshToken string `json:"refresh_token"`
}

func (cfg *apiConfig) loginUserHandler(w http.ResponseWriter, r *http.Request) {

	decoder := json.NewDecoder(r.Body)
	userLoginReq := userLoginRequest{}
	err := decoder.Decode(&userLoginReq)
	if err != nil {
		log.Printf("Error decoding parameters: %s", err)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}
	if len(userLoginReq.Password) <= 0 {
		respBody := chirpyResponseError{
			Error: "Password can never be empty",
		}
		cfg.responseWithJSON(http.StatusBadRequest, respBody, w, r)
	} else if len(userLoginReq.Email) <= 0 {
		respBody := chirpyResponseError{
			Error: "Email is empty",
		}
		cfg.responseWithJSON(http.StatusBadRequest, respBody, w, r)
	} else {
		expiry_duration, err := time.ParseDuration("3600s") // one hour
		if err != nil {
			log.Printf("Error creating duration: %s", err)
			w.WriteHeader(http.StatusInternalServerError)
		}
		user, err := cfg.dbQueries.GetUser(r.Context(), userLoginReq.Email)
		if err != nil {
			if err == sql.ErrNoRows {
				// log.Printf("No user found")
				w.WriteHeader(http.StatusUnauthorized)
			} else {
				log.Printf("Error finding user: %s", err)
				w.WriteHeader(http.StatusInternalServerError)
			}
			return
		}
		matches_pw, err2 := auth.CheckPasswordHash(userLoginReq.Password, user.HashedPassword)
		if err2 != nil {
			log.Printf("Error checking password hash: %s", err2)
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
		if !matches_pw {
			// log.Printf("Invalid password")
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
		new_token, err := auth.MakeJWT(user.ID, cfg.jwt_secret, expiry_duration)
		if err != nil {
			log.Printf("Error creating token: %s", err)
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
		new_refresh_token, err := cfg.GetRefreshToken(user.ID, r)
		if err != nil {
			log.Printf("Error creating refresh token: %s", err)
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
		respBody := userLoginResponseValid{
			Id:           user.ID.String(),
			CreatedAt:    user.CreatedAt.Format(time.UnixDate),
			UpdatedAt:    user.UpdatedAt.Format(time.UnixDate),
			Email:        user.Email,
			Token:        new_token,
			RefreshToken: new_refresh_token,
		}
		cfg.responseWithJSON(http.StatusOK, respBody, w, r)
	}
}

func (cfg *apiConfig) createUserHandler(w http.ResponseWriter, r *http.Request) {

	decoder := json.NewDecoder(r.Body)
	userCreateReq := userCreateRequest{}
	err := decoder.Decode(&userCreateReq)
	if err != nil {
		log.Printf("Error decoding parameters: %s", err)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}
	if len(userCreateReq.Password) <= 0 {
		respBody := chirpyResponseError{
			Error: "Password can never be empty",
		}
		cfg.responseWithJSON(http.StatusBadRequest, respBody, w, r)
	} else if len(userCreateReq.Email) <= 0 {
		respBody := chirpyResponseError{
			Error: "Email is empty",
		}
		cfg.responseWithJSON(http.StatusBadRequest, respBody, w, r)
	} else {
		hashed_pw, err2 := auth.HashPassword(userCreateReq.Password)
		if err2 != nil {
			log.Printf("Error hashing password: %s", err2)
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
		userCreateParms := database.CreateUserParams{
			Email:          userCreateReq.Email,
			HashedPassword: hashed_pw,
		}
		user, err := cfg.dbQueries.CreateUser(r.Context(), userCreateParms)
		if err != nil {
			pqErr := pq.As(err, pqerror.UniqueViolation)
			if pqErr != nil {
				error_text := fmt.Sprintf("user %q already exists", userCreateReq.Email)
				log.Println(error_text)
				respBody := chirpyResponseError{
					Error: error_text,
				}
				cfg.responseWithJSON(http.StatusBadRequest, respBody, w, r)
			} else {
				log.Printf("Error creating user: %s", err)
				w.WriteHeader(http.StatusInternalServerError)
			}
			return
		}
		respBody := userCreateResponseValid{
			Id:        user.ID.String(),
			CreatedAt: user.CreatedAt.Format(time.UnixDate),
			UpdatedAt: user.UpdatedAt.Format(time.UnixDate),
			Email:     user.Email,
		}
		cfg.responseWithJSON(http.StatusCreated, respBody, w, r)
	}
}

func (cfg *apiConfig) GetRefreshToken(user_id uuid.UUID, r *http.Request) (string, error) {
	r_token, err := cfg.dbQueries.GetRefreshTokenByUserID(r.Context(), user_id)
	if err != nil {
		if err != sql.ErrNoRows {
			return "", fmt.Errorf("Error finding refresh token: %s", err)
		}
	} else {
		if r_token.ExpiresAt.After(time.Now().UTC()) {
			return r_token.Token, nil
		}
	}
	new_token := auth.MakeRefreshToken()
	createRefreshTokenParams := database.CreateRefreshTokenParams{
		Token:     new_token,
		UserID:    user_id,
		ExpiresAt: time.Now().UTC().Add(60 * 24 * time.Hour),
		RevokedAt: sql.NullTime{
			Valid: false,
		},
	}
	r_token, err = cfg.dbQueries.CreateRefreshToken(r.Context(), createRefreshTokenParams)
	if err != nil {
		return "", fmt.Errorf("Error creating refresh token: %s", err)
	}
	return r_token.Token, nil
}
