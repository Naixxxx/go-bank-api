package middleware

import (
	"context"
	"net/http"
	"strconv"
	"strings"

	"github.com/golang-jwt/jwt/v5"
)

type ctxKey string

const userIDKey ctxKey = "userID"

func UserID(r *http.Request) (int64, bool) {
	v, ok := r.Context().Value(userIDKey).(int64)

	return v, ok
}

func Auth(secret []byte) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			header := r.Header.Get("Authorization")
			if header == "" || !strings.HasPrefix(header, "Bearer ") {
				http.Error(w, "Authorization header required", http.StatusUnauthorized)

				return
			}

			claims := &jwt.RegisteredClaims{}
			token, err := jwt.ParseWithClaims(strings.TrimPrefix(header, "Bearer "), claims, func(t *jwt.Token) (any, error) { return secret, nil })
			if err != nil || !token.Valid {
				http.Error(w, "Invalid token", http.StatusUnauthorized)
				return
			}

			id, err := strconv.ParseInt(claims.Subject, 10, 64)
			if err != nil {
				http.Error(w, "Invalid token subject", http.StatusUnauthorized)
				return
			}

			next.ServeHTTP(w, r.WithContext(context.WithValue(r.Context(), userIDKey, id)))
		})
	}
}
