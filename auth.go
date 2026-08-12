package main

import (
	"context"
	"net/http"
	"os"
	"strings"

	"github.com/golang-jwt/jwt/v5"
)

type C_Claims struct {
	JuryTeamID int `json:"jury_team_id"`
	jwt.RegisteredClaims
}

func c_getJWTSecret() []byte {
	secret := os.Getenv("JWT_SECRET")
	if secret == "" {
		return []byte("super_secret_local_dev_key")
	}
	return []byte(secret)
}

func C_AuthMiddleware(next http.Handler) http.Handler {

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		auth := r.Header.Get("Authorization")
		if !strings.HasPrefix(auth, "Bearer ") {
			http.Error(w, `{"error": "Missing Token"}`, http.StatusUnauthorized)
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
			http.Error(w, err.Error(), http.StatusUnauthorized)
			return
		}

		ctx := context.WithValue(r.Context(), juryTeamIDKey, claims.JuryTeamID)
		next.ServeHTTP(w, r.WithContext(ctx))

	})
}
