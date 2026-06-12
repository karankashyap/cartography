package main

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"os"
	"path/filepath"
	"strings"
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

const maxUploadSize = 50 << 20 // 50 MB

func main() {
	dbURL := mustEnv("DATABASE_URL")
	chatDBURL := envOrDefault("CHAT_DATABASE_URL", dbURL)
	ollamaURL := envOrDefault("OLLAMA_URL", "http://localhost:11434")
	port := envOrDefault("PORT", "8080")
	uploadDir := envOrDefault("UPLOAD_DIR", os.TempDir()+"/cartograph-uploads")

	if err := os.MkdirAll(uploadDir, 0o755); err != nil {
		slog.Error("create upload dir", "err", err)
		os.Exit(1)
	}

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
	pubsub.Listen(ctx, "import_progress")

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
	mux.HandleFunc("/health", healthHandler(pool))
	mux.HandleFunc("/upload", uploadHandler(uploadDir))

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

func healthHandler(pool interface{ Ping(context.Context) error }) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if err := pool.Ping(r.Context()); err != nil {
			http.Error(w, "db unavailable", http.StatusServiceUnavailable)
			return
		}
		w.WriteHeader(http.StatusOK)
	}
}

func uploadHandler(uploadDir string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}

		if err := r.ParseMultipartForm(maxUploadSize); err != nil {
			http.Error(w, "file too large or invalid form", http.StatusBadRequest)
			return
		}

		file, header, err := r.FormFile("file")
		if err != nil {
			http.Error(w, "missing file field", http.StatusBadRequest)
			return
		}
		defer file.Close()

		ext := strings.ToLower(filepath.Ext(header.Filename))
		if ext != ".csv" {
			http.Error(w, "only CSV files accepted", http.StatusBadRequest)
			return
		}

		// Use timestamp-prefixed filename to avoid collisions
		filename := fmt.Sprintf("%d_%s", time.Now().UnixNano(), sanitizeFilename(header.Filename))
		dst, err := os.Create(filepath.Join(uploadDir, filename))
		if err != nil {
			http.Error(w, "server error", http.StatusInternalServerError)
			return
		}
		defer dst.Close()

		if _, err := io.Copy(dst, io.LimitReader(file, maxUploadSize)); err != nil {
			http.Error(w, "write error", http.StatusInternalServerError)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		fmt.Fprintf(w, `{"filename":%q}`, filename)
	}
}

func sanitizeFilename(name string) string {
	name = filepath.Base(name)
	var b strings.Builder
	for _, c := range name {
		if (c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z') || (c >= '0' && c <= '9') || c == '.' || c == '-' || c == '_' {
			b.WriteRune(c)
		} else {
			b.WriteRune('_')
		}
	}
	return b.String()
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
