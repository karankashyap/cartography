package main

import (
	"context"
	"log/slog"
	"net/http"
	"os"
	"time"

	"github.com/99designs/gqlgen/graphql/handler"
	"github.com/99designs/gqlgen/graphql/handler/extension"
	"github.com/99designs/gqlgen/graphql/handler/transport"
	"github.com/99designs/gqlgen/graphql/playground"
	"github.com/gorilla/websocket"
	"github.com/rs/cors"

	"cartograph/api/graph"
	"cartograph/api/graph/generated"
	"cartograph/api/internal/ai"
	"cartograph/api/internal/db"
)

func main() {
	dbURL := mustEnv("DATABASE_URL")
	chatDBURL := envOrDefault("CHAT_DATABASE_URL", dbURL)
	ollamaURL := envOrDefault("OLLAMA_URL", "http://localhost:11434")
	port := envOrDefault("PORT", "8080")

	ctx := context.Background()

	pool, err := db.Connect(ctx, dbURL)
	if err != nil {
		slog.Error("db connect failed", "err", err)
		os.Exit(1)
	}
	defer pool.Close()

	if err := db.Migrate(pool); err != nil {
		slog.Error("migration failed", "err", err)
		os.Exit(1)
	}

	chatPool, err := db.Connect(ctx, chatDBURL)
	if err != nil {
		slog.Error("chat db connect failed", "err", err)
		os.Exit(1)
	}
	defer chatPool.Close()

	pubsub := db.NewPubSub(pool)

	aiClient := ai.NewClient(ollamaURL, chatPool)

	resolver := &graph.Resolver{
		DB:       pool,
		ChatDB:   chatPool,
		AIClient: aiClient,
		PubSub:   pubsub,
	}

	srv := handler.New(generated.NewExecutableSchema(generated.Config{
		Resolvers: resolver,
	}))

	srv.AddTransport(transport.Options{})
	srv.AddTransport(transport.GET{})
	srv.AddTransport(transport.POST{})
	srv.AddTransport(transport.MultipartForm{})
	srv.AddTransport(&transport.Websocket{
		KeepAlivePingInterval: 10 * time.Second,
		Upgrader: websocket.Upgrader{
			CheckOrigin: func(r *http.Request) bool { return true },
		},
	})

	srv.Use(extension.Introspection{})

	mux := http.NewServeMux()
	mux.Handle("/query", srv)
	mux.Handle("/", playground.Handler("Cartograph", "/query"))
	mux.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		if err := pool.Ping(r.Context()); err != nil {
			http.Error(w, "db unavailable", http.StatusServiceUnavailable)
			return
		}
		w.WriteHeader(http.StatusOK)
	})

	c := cors.New(cors.Options{
		AllowedOrigins:   []string{"http://localhost:3000", "http://localhost:8081"},
		AllowedHeaders:   []string{"*"},
		AllowedMethods:   []string{"GET", "POST", "OPTIONS"},
		AllowCredentials: true,
	})

	slog.Info("api listening", "port", port)
	if err := http.ListenAndServe(":"+port, c.Handler(mux)); err != nil {
		slog.Error("server error", "err", err)
		os.Exit(1)
	}
}

func mustEnv(key string) string {
	v := os.Getenv(key)
	if v == "" {
		slog.Error("required env var not set", "key", key)
		os.Exit(1)
	}
	return v
}

func envOrDefault(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}
