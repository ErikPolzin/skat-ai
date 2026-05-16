package server

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"

	"skat/server/db"

	"golang.org/x/crypto/bcrypt"
)

type profileContextKey struct{}

type profileResponse struct {
	PlayerID    string `json:"player_id"`
	PlayerName  string `json:"player_name"`
	ProfileIcon string `json:"profile_icon"`
}

func profileToResponse(profile *db.ProfileEntry) profileResponse {
	return profileResponse{
		PlayerID:    profile.ID,
		PlayerName:  profile.Name,
		ProfileIcon: profile.ProfileIcon,
	}
}

func (s *Server) basicAuthMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodOptions {
			next.ServeHTTP(w, r)
			return
		}

		username, password, ok := r.BasicAuth()
		if !ok || strings.TrimSpace(username) == "" || password == "" {
			writeAuthRequired(w)
			return
		}

		profile, err := s.authenticateProfile(username, password)
		if err != nil {
			writeAuthRequired(w)
			return
		}

		ctx := context.WithValue(r.Context(), profileContextKey{}, profile)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

func (s *Server) authenticateProfile(username, password string) (*db.ProfileEntry, error) {
	profile, err := s.db.GetProfileByName(strings.TrimSpace(username))
	if err != nil {
		return nil, err
	}
	if profile.IsAgent || profile.PasswordHash == "" {
		return nil, fmt.Errorf("profile cannot authenticate")
	}
	if err := bcrypt.CompareHashAndPassword([]byte(profile.PasswordHash), []byte(password)); err != nil {
		return nil, err
	}
	return profile, nil
}

func currentProfile(r *http.Request) (*db.ProfileEntry, error) {
	profile, ok := r.Context().Value(profileContextKey{}).(*db.ProfileEntry)
	if !ok || profile == nil {
		return nil, fmt.Errorf("authenticated profile required")
	}
	return profile, nil
}

func currentProfileID(r *http.Request) (string, error) {
	profile, err := currentProfile(r)
	if err != nil {
		return "", err
	}
	return profile.ID, nil
}

func writeAuthRequired(w http.ResponseWriter) {
	w.Header().Set("WWW-Authenticate", `Basic realm="skat"`)
	http.Error(w, "authentication required", http.StatusUnauthorized)
}

func hashPassword(password string) (string, error) {
	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return "", err
	}
	return string(hash), nil
}

func passwordHashMatches(hash, password string) bool {
	return bcrypt.CompareHashAndPassword([]byte(hash), []byte(password)) == nil
}

func writeJSON(w http.ResponseWriter, v any) {
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(v)
}
