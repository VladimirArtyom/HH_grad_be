package main

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"strings"

	"github.com/golang-jwt/jwt/v5"
)

type C_Claims struct {
	JuryTeamID int    `json:"jury_team_id"`
	Role       string `json:"jury_role"`
	jwt.RegisteredClaims
}

func c_getJWTSecret() []byte {
	secret := os.Getenv("JWT_SECRET")
	if secret == "" {
		return []byte("super_secret_local_dev_key")
	}
	return []byte(secret)
}

func sendJSONError(w http.ResponseWriter, message string, status int) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)

	fmt.Fprintf(w, `{"error": %s}`, message)
}

func C_AuthMiddleware(next http.Handler) http.Handler {

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		auth := r.Header.Get("Authorization")
		if auth == "" {
			sendJSONError(w, "Missing Authorization header", http.StatusUnauthorized)
			return
		}
		if !strings.HasPrefix(auth, "Bearer ") {
			sendJSONError(w, "Invalid Authorization format", http.StatusUnauthorized)
			return
		}

		// CheckJWT
		token_string := strings.TrimPrefix(auth, "Bearer ")
		jwtSecret := c_getJWTSecret()
		claims := &C_Claims{}

		token, err := jwt.ParseWithClaims(token_string, claims, func(t *jwt.Token) (interface{}, error) {
			return jwtSecret, nil
		})
		if err != nil || !token.Valid {
			sendJSONError(w, "Invalid or expired token", http.StatusUnauthorized)
			return
		}

		ctx := context.WithValue(r.Context(), roleKey, claims.Role)
		ctx = context.WithValue(ctx, juryTeamIDKey, claims.JuryTeamID)
		next.ServeHTTP(w, r.WithContext(ctx))

	})
}
