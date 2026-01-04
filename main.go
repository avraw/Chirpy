package main

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"strings"
	"sync/atomic"
	"time"

	"github.com/avraw/Chirpy/internal/database"
	"github.com/google/uuid"
	"github.com/joho/godotenv"
	_ "github.com/lib/pq"
)

type User struct {
	ID        uuid.UUID `json:"id"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
	Email     string    `json:"email"`
}

type apiConfig struct {
	fileServerHits atomic.Int32
	dbq            *database.Queries
	platform       string
}

func (a *apiConfig) cfgApiConfig(next http.Handler) http.Handler {

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		a.fileServerHits.Add(1)
		next.ServeHTTP(w, r)
	})
}
func healthzHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	w.WriteHeader(200)
	w.Write([]byte("OK"))
}

func (a *apiConfig) metricsHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.WriteHeader(200)
	w.Write([]byte(fmt.Sprintf(`<html>
  <body>
    <h1>Welcome, Chirpy Admin</h1>
    <p>Chirpy has been visited %d times!</p>
  </body>
</html>`, (int(a.fileServerHits.Load())))))
}

func validateHandler(w http.ResponseWriter, r *http.Request) {

	type request struct {
		Body string `json:"body"`
	}
	type errorResponse struct {
		Error string `json:"error"`
	}

	type cleanedResponse struct {
		Valid string `json:"cleaned_body"`
	}
	req := request{}

	cleanRes := cleanedResponse{}
	decoder := json.NewDecoder(r.Body)

	err := decoder.Decode(&req)
	if err != nil {

		respondWithError(w, 500, "Something went wrong")
		return
	}

	if len(req.Body) > 140 {

		respondWithError(w, 400, "Chirp is too long")

		return
	}

	cleanRes.Valid = badWordReplacer(req.Body)
	respondWithJSON(w, 200, cleanRes)

}

func badWordReplacer(msg string) string {

	s := strings.Split(msg, " ")

	for i, value := range s {
		value = strings.ToLower(value)
		if value == "kerfuffle" || value == "sharbert" || value == "fornax" {
			value = "****"
			s[i] = value
		}

	}
	return strings.Join(s, " ")
}

func (a *apiConfig) createUser(w http.ResponseWriter, r *http.Request) {

	type request struct {
		Email string `json:"email"`
	}
	req := request{}
	decoder := json.NewDecoder(r.Body)
	err := decoder.Decode(&req)
	if err != nil {

		respondWithError(w, 500, "Something went wrong")
		return
	}

	user, err := a.dbq.CreateUser(r.Context(), req.Email)
	if err != nil {

		respondWithError(w, 500, fmt.Sprintf("Something went wrong : %s ", err))
		return
	}
	var u User

	u.ID = user.ID
	u.CreatedAt = user.CreatedAt
	u.Email = user.Email
	u.UpdatedAt = user.UpdatedAt

	respondWithJSON(w, 201, u)
}
func (a *apiConfig) resetHandler(w http.ResponseWriter, r *http.Request) {

	if a.platform != "dev" {
		respondWithError(w, 403, "Now Allowed")
	}
	err := a.dbq.DeleteUser(r.Context())
	if err != nil {
		respondWithError(w, 500, fmt.Sprintf("Error deleting all users : %s ", err))
	}
	respondWithJSON(w, 200, "")
}

//helper functions for validate request body

func respondWithJSON(w http.ResponseWriter, code int, payload interface{}) {

	data, err := json.Marshal(payload)
	if err != nil {

		respondWithError(w, 500, fmt.Sprintf("Error marshalling JSON: %s", err))

		return
	}
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(code)
	w.Write(data)
}

func respondWithError(w http.ResponseWriter, code int, msg string) {

	type errorResponse struct {
		Error string `json:"error"`
	}

	errRes := errorResponse{}
	errRes.Error = msg

	data, err := json.Marshal(errRes)
	if err != nil {
		errRes.Error = fmt.Sprintf("Error marshalling JSON: %s", err)
		w.Write([]byte(errRes.Error))
		w.WriteHeader(500)
		return
	}

	w.Header().Set("Content-Type", "application/json; charset=utf-8")

	w.WriteHeader(code)
	w.Write(data)
}

func main() {
	godotenv.Load()

	dbURL := os.Getenv("DB_URL")
	plat := os.Getenv("PLATFORM")
	db, err := sql.Open("postgres", dbURL)
	if err != nil {
		fmt.Printf("Error connecting to db: %s ", err)
	}

	dbQueries := database.New(db)

	a := apiConfig{
		dbq:      dbQueries,
		platform: plat,
	}
	mux := http.NewServeMux()

	dh := http.Dir(".")
	fh := http.FileServer(dh)

	mux.Handle("/app/", a.cfgApiConfig(http.StripPrefix("/app", fh)))

	mux.HandleFunc("GET /api/healthz", healthzHandler)

	mux.HandleFunc("GET /admin/metrics", a.metricsHandler)

	mux.HandleFunc("POST /admin/reset", a.resetHandler)

	mux.HandleFunc("POST /api/validate_chirp", validateHandler)

	mux.HandleFunc("POST /api/users", a.createUser)

	s := http.Server{

		Addr:    ":8080",
		Handler: mux,
	}

	s.ListenAndServe()
	fmt.Println("Hello world")

}
