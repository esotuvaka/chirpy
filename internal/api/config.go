package api

import (
	"chirpy/internal/database"
	"sync/atomic"
)

type Config struct {
	FileserverHits   atomic.Int32
	DbQueries        *database.Queries
	Platform         string
	JwtSigningSecret string
	PolkaKey         string
}
