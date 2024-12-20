package api

import (
	"database/sql"
	"encoding/json"
	"log"
	"net/http"

	"github.com/google/uuid"
)

func (cfg *Config) UpgradeUser(w http.ResponseWriter, r *http.Request) {
	type Data struct {
		UserId string `json:"user_id"`
	}
	type parameters struct {
		Event string `json:"event"`
		Data  Data   `json:"data"`
	}

	decoder := json.NewDecoder(r.Body)
	params := parameters{}
	err := decoder.Decode(&params)
	if err != nil {
		log.Printf("decoding parameters: %s", err)
		w.WriteHeader(http.StatusBadRequest)
		w.Write([]byte("BAD REQUEST"))
		return
	}

	if params.Event != "user.upgraded" {
		log.Printf("ignoring event: %s", params.Event)
		w.WriteHeader(http.StatusNoContent)
		w.Write([]byte("NO CONTENT"))
		return
	}

	userUUID, err := uuid.Parse(params.Data.UserId)
	if err != nil {
		log.Printf("parsing uuid: %s", err)
		w.WriteHeader(http.StatusInternalServerError)
		w.Write([]byte("INTERNAL SERVER ERROR"))
		return
	}

	err = cfg.DbQueries.UpgradeUser(r.Context(), userUUID)
	if err != nil {
		if err == sql.ErrNoRows {
			log.Printf("unable to find user for upgrade: %s", userUUID)
			w.WriteHeader(http.StatusNotFound)
			w.Write([]byte("NOT FOUND"))
			return
		}
		log.Printf("upgrading user in db: %s", err)
		w.WriteHeader(http.StatusInternalServerError)
		w.Write([]byte("INTERNAL SERVER ERROR"))
		return
	}

	w.WriteHeader(http.StatusNoContent)
}
