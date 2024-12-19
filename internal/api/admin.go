package api

import (
	"fmt"
	"net/http"
)

func (cfg *Config) PageHits(w http.ResponseWriter, r *http.Request) {
	resp := fmt.Sprintf(`<html>
            <body>
                <h1>Welcome, Chirpy Admin</h1>
                <p>Chirpy has been visited %d times!</p>
            </body>
        </html>`, cfg.FileserverHits.Load())
	w.Write([]byte(resp))
}

func (cfg *Config) ResetHitsAndUsers(w http.ResponseWriter, r *http.Request) {
	if cfg.Platform != "dev" {
		w.WriteHeader(http.StatusForbidden)
		return
	}
	cfg.DbQueries.DeleteAllUsers(r.Context())
	cfg.FileserverHits.Swap(0)
	w.Write([]byte("OK"))
}
