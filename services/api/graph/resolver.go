package graph

import (
	"cartograph/api/internal/ai"
	"cartograph/api/internal/db"
	"github.com/jackc/pgx/v5/pgxpool"
)

// Resolver is the root resolver. All query/mutation/subscription resolvers
// are implemented in *.resolvers.go files generated alongside this file.
type Resolver struct {
	DB       *pgxpool.Pool
	ChatDB   *pgxpool.Pool // read-only cartograph_chat role
	AIClient *ai.Client
	PubSub   *db.PubSub
}
