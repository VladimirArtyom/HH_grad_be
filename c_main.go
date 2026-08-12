package main

import (
	"fmt"
	"log"
	"net/http"
)

type DBSecret struct {
	Username string `json:"username`
	Password string `json:"password"`
	Host     string `json:"host"`
	Port     int    `json:"port"`
	DBName   string `json:"dbname"`
}

func CORSMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "*") // Restrict to your frontend URL in prod
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization")
		if r.Method == "OPTIONS" {
			w.WriteHeader(http.StatusOK)
			return
		}
		next.ServeHTTP(w, r)
	})
}

// Use this code snippet in your app.
// If you need more information about configurations or implementing the sample code, visit the AWS docs:
// https://aws.github.io/aws-sdk-go-v2/docs/getting-started/

func main() {
	Connect()

	mux := http.NewServeMux()
	mux.Handle("POST /api/login", http.HandlerFunc(C_Login))

	mux.Handle("GET /api/teams", http.HandlerFunc(C_GetTeamsHandler))
	//mux.Handle("GET /api/grades/{id}", C_AuthMiddleware(http.HandlerFunc(C_GetGradesHandler)))
	mux.Handle("POST /api/jury/grades/{id}", C_AuthMiddleware(http.HandlerFunc(C_SaveGradesHandler)))

	mux.Handle("GET /api/jury/grades", C_AuthMiddleware(http.HandlerFunc(C_GetAllGradesJury)))

	mux.Handle("GET /api/export/grades", C_AuthMiddleware(http.HandlerFunc(ExportGradesHandler)))

	log.Printf("Server running on http://localhost:8081")
	err := http.ListenAndServe(":8081", CORSMiddleware(mux))
	if err != nil {
		fmt.Println(err)
	}
}
