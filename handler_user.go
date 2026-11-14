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

type userUpdateRequest struct {
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

type userRefreshResponseValid struct {
	Token string `json:"token"`
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
		expiry_duration, err := getAccessJWTExpiryDuration()
		if err != nil {
			log.Printf("Error creating duration: %s", err)
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
		user, err := cfg.dbQueries.GetUser(r.Context(), userLoginReq.Email)
		if err != nil {
			if err == sql.ErrNoRows {
				respBody := chirpyResponseError{
					Error: fmt.Sprintf("Invalid user: %v", err),
				}
				cfg.responseWithJSON(http.StatusUnauthorized, respBody, w, r)
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
			respBody := chirpyResponseError{
				Error: fmt.Sprintf("Invalid password: %v", err),
			}
			cfg.responseWithJSON(http.StatusUnauthorized, respBody, w, r)
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

func getAccessJWTExpiryDuration() (time.Duration, error) {
	expiry_duration, err := time.ParseDuration("3600s") // one hour
	if err != nil {
		return 0, err
	}
	return expiry_duration, nil
}

func (cfg *apiConfig) updateUserHandler(w http.ResponseWriter, r *http.Request) {
	// Make sure user is logged in and has an access token
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
	userUpdateReq := userUpdateRequest{}
	if err := decoder.Decode(&userUpdateReq); err != nil {
		log.Printf("Error decoding parameters: %s", err)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}
	if len(userUpdateReq.Password) <= 0 {
		respBody := chirpyResponseError{
			Error: "Password can never be empty",
		}
		cfg.responseWithJSON(http.StatusBadRequest, respBody, w, r)
	} else if len(userUpdateReq.Email) <= 0 {
		respBody := chirpyResponseError{
			Error: "Email is empty",
		}
		cfg.responseWithJSON(http.StatusBadRequest, respBody, w, r)
	} else {
		// Hash the new password
		hashed_pw, err2 := auth.HashPassword(userUpdateReq.Password)
		if err2 != nil {
			log.Printf("Error hashing password: %s", err2)
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
		// Update the users record
		userUpdateParms := database.UpdateUserByIDParams{
			Email:          userUpdateReq.Email,
			HashedPassword: hashed_pw,
			ID:             passed_uuid,
		}
		if err := cfg.dbQueries.UpdateUserByID(r.Context(), userUpdateParms); err != nil {
			pqErr := pq.As(err, pqerror.UniqueViolation)
			if pqErr != nil {
				error_text := fmt.Sprintf("user email %q already exists", userUpdateReq.Email)
				log.Println(error_text)
				respBody := chirpyResponseError{
					Error: error_text,
				}
				cfg.responseWithJSON(http.StatusBadRequest, respBody, w, r)
			} else {
				log.Printf("Error updating user: %s", err)
				w.WriteHeader(http.StatusInternalServerError)
			}
			return
		}
		// retreive the updated users record
		user, err := cfg.dbQueries.GetUser(r.Context(), userUpdateParms.Email)
		if err != nil {
			if err == sql.ErrNoRows {
				// log.Printf("No user found")
				respBody := chirpyResponseError{
					Error: fmt.Sprintf("Invalid user: %v", err),
				}
				cfg.responseWithJSON(http.StatusUnauthorized, respBody, w, r)
			} else {
				log.Printf("Error finding user: %s", err)
				w.WriteHeader(http.StatusInternalServerError)
			}
			return
		}
		// create a new access and refresh token
		expiry_duration, err := getAccessJWTExpiryDuration()
		if err != nil {
			log.Printf("Error creating duration: %s", err)
			w.WriteHeader(http.StatusInternalServerError)
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
			Id:        user.ID.String(),
			CreatedAt: user.CreatedAt.Format(time.UnixDate),
			UpdatedAt: user.UpdatedAt.Format(time.UnixDate),
			Email:     user.Email,
			Token:     new_token,
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
				error_text := fmt.Sprintf("user email %q already exists", userCreateReq.Email)
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

func (cfg *apiConfig) refreshUserHandler(w http.ResponseWriter, r *http.Request) {
	user_id := uuid.Nil
	token, err := auth.GetBearerToken(r.Header)
	if err != nil {
		log.Printf("Error reading refresh token: %s", err)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}
	r_token, err := cfg.dbQueries.GetRefreshToken(r.Context(), token)
	if err != nil {
		if err != sql.ErrNoRows {
			log.Printf("DB Error finding refresh token: %s", err)
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
	} else {
		if r_token.ExpiresAt.After(time.Now().UTC()) && (!r_token.RevokedAt.Valid || r_token.RevokedAt.Time.After(time.Now().UTC())) {
			user_id = r_token.UserID
		}
	}
	if user_id == uuid.Nil {
		respBody := chirpyResponseError{
			Error: fmt.Sprintf("Invalid authentication token: %v", err),
		}
		cfg.responseWithJSON(http.StatusUnauthorized, respBody, w, r)
		return
	}
	expiry_duration, err := getAccessJWTExpiryDuration()
	if err != nil {
		log.Printf("Error creating duration: %s", err)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}
	new_token, err := auth.MakeJWT(user_id, cfg.jwt_secret, expiry_duration)
	if err != nil {
		log.Printf("Error creating token: %s", err)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}
	respBody := userRefreshResponseValid{
		Token: new_token,
	}
	cfg.responseWithJSON(http.StatusOK, respBody, w, r)
}

func (cfg *apiConfig) revokeUserHandler(w http.ResponseWriter, r *http.Request) {
	token, err := auth.GetBearerToken(r.Header)
	if err != nil {
		log.Printf("Error reading refresh token: %s", err)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}
	if err := cfg.dbQueries.RevokeRefreshToken(r.Context(), token); err != nil {
		if err != sql.ErrNoRows {
			log.Printf("DB Error revoking refresh token: %s", err)
			w.WriteHeader(http.StatusInternalServerError)
			return
		} else {
			respBody := chirpyResponseError{
				Error: fmt.Sprintf("Invalid authentication token: %v", err),
			}
			cfg.responseWithJSON(http.StatusUnauthorized, respBody, w, r)
			return
		}
	}
	w.WriteHeader(http.StatusNoContent)
}
