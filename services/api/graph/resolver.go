package graph

import (
	"cartograph/api/graph/model"
	"cartograph/api/internal/ai"
	"cartograph/api/internal/db"
	"github.com/jackc/pgx/v5/pgxpool"
)

// Resolver is the root resolver. All query/mutation/subscription resolvers
// are implemented in *.resolvers.go files generated alongside this file.
type Resolver struct {
	DB        *pgxpool.Pool
	ChatDB    *pgxpool.Pool // read-only cartograph_chat role
	Providers map[string]*ai.Client
	PubSub    *db.PubSub
}

// aiClient returns the AI client for the given provider, defaulting to OLLAMA.
func (r *Resolver) aiClient(provider *model.AIProvider) *ai.Client {
	if provider != nil && *provider == model.AIProviderLmstudio {
		if c, ok := r.Providers[string(model.AIProviderLmstudio)]; ok {
			return c
		}
	}
	return r.Providers[string(model.AIProviderOllama)]
}
