package main

import (
	"context"
	"encoding/json"
	"log"
	"net/http"
	"os"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/jackc/pgx/v5/pgxpool"
)

type Bookmark struct {
	ID        int       `json:"id"`
	URL       string    `json:"url"`
	Title     string    `json:"title"`
	CreatedAt time.Time `json:"created_at"`
}

type bookmarkStore struct {
	pool *pgxpool.Pool
}

func (s *bookmarkStore) Add(ctx context.Context, url, title string) (Bookmark, error) {
	var b Bookmark
	err := s.pool.QueryRow(ctx,
		`INSERT INTO bookmarks (url, title) VALUES ($1, $2) RETURNING id, url, title, created_at`,
		url, title,
	).Scan(&b.ID, &b.URL, &b.Title, &b.CreatedAt)
	if err != nil {
		return Bookmark{}, err
	}
	return b, nil
}

func (s *bookmarkStore) List(ctx context.Context) ([]Bookmark, error) {
	rows, err := s.pool.Query(ctx,
		`SELECT id, url, title, created_at FROM bookmarks ORDER BY created_at DESC`,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	bookmarks := []Bookmark{}
	for rows.Next() {
		var b Bookmark
		if err := rows.Scan(&b.ID, &b.URL, &b.Title, &b.CreatedAt); err != nil {
			return nil, err
		}
		bookmarks = append(bookmarks, b)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return bookmarks, nil
}

type createBookmarkRequest struct {
	URL   string `json:"url"`
	Title string `json:"title"`
}

func main() {
	dbURL := os.Getenv("DATABASE_URL")
	if dbURL == "" {
		dbURL = "postgres://bookmark:localdev@localhost:5432/bookmarks?sslmode=disable"
	}

	pool, err := pgxpool.New(context.Background(), dbURL)
	if err != nil {
		log.Fatalf("failed to create connection pool: %v", err)
	}
	defer pool.Close()

	if err := pool.Ping(context.Background()); err != nil {
		log.Fatalf("failed to ping database: %v", err)
	}
	log.Println("connected to postgres")

	store := &bookmarkStore{pool: pool}

	r := chi.NewRouter()
	r.Use(middleware.Logger)
	r.Use(middleware.Recoverer)

	r.Get("/", func(w http.ResponseWriter, req *http.Request) {
		w.Write([]byte("Bookmark service is running\n"))
	})

	r.Get("/health", func(w http.ResponseWriter, req *http.Request) {
		w.Write([]byte("ok\n"))
	})

	r.Post("/bookmarks", func(w http.ResponseWriter, req *http.Request) {
		var body createBookmarkRequest
		if err := json.NewDecoder(req.Body).Decode(&body); err != nil {
			http.Error(w, "invalid JSON: "+err.Error(), http.StatusBadRequest)
			return
		}
		if body.URL == "" {
			http.Error(w, "url is required", http.StatusBadRequest)
			return
		}

		bookmark, err := store.Add(req.Context(), body.URL, body.Title)
		if err != nil {
			log.Printf("failed to add bookmark: %v", err)
			http.Error(w, "internal server error", http.StatusInternalServerError)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusCreated)
		json.NewEncoder(w).Encode(bookmark)
	})

	r.Get("/bookmarks", func(w http.ResponseWriter, req *http.Request) {
		bookmarks, err := store.List(req.Context())
		if err != nil {
			log.Printf("failed to list bookmarks: %v", err)
			http.Error(w, "internal server error", http.StatusInternalServerError)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(bookmarks)
	})

	addr := ":8080"
	log.Printf("Listening on %s", addr)
	if err := http.ListenAndServe(addr, r); err != nil {
		log.Fatal(err)
	}
}
