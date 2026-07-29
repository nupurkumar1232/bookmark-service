package main

import (
	"context"
	"encoding/json"
	"errors"
	"log"
	"net/http"
	"os"
	"os/signal"
	"strconv"
	"syscall"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/golang-jwt/jwt/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/joho/godotenv"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"golang.org/x/crypto/bcrypt"
)

var (
	httpRequestsTotal = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "http_requests_total",
			Help: "Total number of HTTP requests, labeled by method, path, and status code",
		},
		[]string{"method", "path", "status"},
	)

	httpRequestDuration = promauto.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "http_request_duration_seconds",
			Help:    "HTTP request latency in seconds, labeled by method and path",
			Buckets: []float64{0.001, 0.005, 0.01, 0.025, 0.05, 0.1, 0.25, 0.5, 1, 2, 5},
		},
		[]string{"method", "path"},
	)

	httpRequestsInFlight = promauto.NewGauge(
		prometheus.GaugeOpts{
			Name: "http_requests_in_flight",
			Help: "Number of HTTP requests currently being processed",
		},
	)
)

// metricsMiddleware records metrics for each HTTP request.
func metricsMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		start := time.Now()
		httpRequestsInFlight.Inc()
		defer httpRequestsInFlight.Dec()

		// Wrap ResponseWriter to capture the status code
		ww := middleware.NewWrapResponseWriter(w, req.ProtoMajor)

		next.ServeHTTP(ww, req)

		// Use the chi route pattern (e.g. "/bookmarks") instead of the full URL
		// so we don't explode cardinality with paths like /bookmarks/123.
		routePattern := chi.RouteContext(req.Context()).RoutePattern()
		if routePattern == "" {
			routePattern = "unknown"
		}

		duration := time.Since(start).Seconds()
		status := strconv.Itoa(ww.Status())

		httpRequestsTotal.WithLabelValues(req.Method, routePattern, status).Inc()
		httpRequestDuration.WithLabelValues(req.Method, routePattern).Observe(duration)
	})
}

type Bookmark struct {
	ID        int       `json:"id"`
	URL       string    `json:"url"`
	Title     string    `json:"title"`
	CreatedAt time.Time `json:"created_at"`
}

type bookmarkStore struct {
	pool *pgxpool.Pool
}

func (s *bookmarkStore) Add(ctx context.Context, userID int, url, title string) (Bookmark, error) {
	var b Bookmark
	err := s.pool.QueryRow(ctx,
		`INSERT INTO bookmarks (user_id, url, title) VALUES ($1, $2, $3) RETURNING id, url, title, created_at`,
		userID, url, title,
	).Scan(&b.ID, &b.URL, &b.Title, &b.CreatedAt)
	if err != nil {
		return Bookmark{}, err
	}
	return b, nil
}

func (s *bookmarkStore) List(ctx context.Context, userID int) ([]Bookmark, error) {
	rows, err := s.pool.Query(ctx,
		`SELECT id, url, title, created_at FROM bookmarks WHERE user_id = $1 ORDER BY created_at DESC`,
		userID,
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

type User struct {
	ID        int       `json:"id"`
	Email     string    `json:"email"`
	CreatedAt time.Time `json:"created_at"`
}

type userStore struct {
	pool *pgxpool.Pool
}

func (s *userStore) Create(ctx context.Context, email, passwordHash string) (User, error) {
	var u User
	err := s.pool.QueryRow(ctx,
		`INSERT INTO users (email, password_hash) VALUES ($1, $2) RETURNING id, email, created_at`,
		email, passwordHash,
	).Scan(&u.ID, &u.Email, &u.CreatedAt)
	if err != nil {
		return User{}, err
	}
	return u, nil
}

// userRecord is what we read from the DB — includes the password hash.
// Distinct from the User struct (which we send to clients).
type userRecord struct {
	ID           int
	Email        string
	PasswordHash string
	CreatedAt    time.Time
}

func (s *userStore) FindByEmail(ctx context.Context, email string) (userRecord, error) {
	var u userRecord
	err := s.pool.QueryRow(ctx,
		`SELECT id, email, password_hash, created_at FROM users WHERE email = $1`,
		email,
	).Scan(&u.ID, &u.Email, &u.PasswordHash, &u.CreatedAt)
	if err != nil {
		return userRecord{}, err
	}
	return u, nil
}

type signupRequest struct {
	Email    string `json:"email"`
	Password string `json:"password"`
}
type loginRequest struct {
	Email    string `json:"email"`
	Password string `json:"password"`
}

type loginResponse struct {
	Token string `json:"token"`
}

// contextKey is a custom type for context keys to avoid collisions
// with other packages' context values.
type contextKey string

const userIDKey contextKey = "user_id"

// authMiddleware returns a chi middleware that validates the JWT in the
// Authorization header and adds the user_id to the request context.
func authMiddleware(jwtSecret string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
			authHeader := req.Header.Get("Authorization")
			if authHeader == "" {
				http.Error(w, "missing authorization header", http.StatusUnauthorized)
				return
			}

			// Expect "Bearer <token>"
			const prefix = "Bearer "
			if len(authHeader) < len(prefix) || authHeader[:len(prefix)] != prefix {
				http.Error(w, "authorization header must be 'Bearer <token>'", http.StatusUnauthorized)
				return
			}
			tokenString := authHeader[len(prefix):]

			// Parse and validate the token
			token, err := jwt.Parse(tokenString, func(t *jwt.Token) (interface{}, error) {
				// Verify the signing algorithm is what we expect
				if _, ok := t.Method.(*jwt.SigningMethodHMAC); !ok {
					return nil, errors.New("unexpected signing method")
				}
				return []byte(jwtSecret), nil
			})
			if err != nil || !token.Valid {
				log.Printf("token validation failed: %v", err)
				http.Error(w, "invalid token", http.StatusUnauthorized)
				return
			}

			claims, ok := token.Claims.(jwt.MapClaims)
			if !ok {
				http.Error(w, "invalid token claims", http.StatusUnauthorized)
				return
			}

			// JWT numbers are decoded as float64 — convert to int
			userIDFloat, ok := claims["user_id"].(float64)
			if !ok {
				http.Error(w, "invalid user_id in token", http.StatusUnauthorized)
				return
			}
			userID := int(userIDFloat)

			// Add user_id to the context for downstream handlers
			ctx := context.WithValue(req.Context(), userIDKey, userID)
			next.ServeHTTP(w, req.WithContext(ctx))
		})
	}
}

// userIDFromContext is a helper for handlers to retrieve the user_id.
func userIDFromContext(ctx context.Context) (int, bool) {
	id, ok := ctx.Value(userIDKey).(int)
	return id, ok
}

func main() {
	// Load .env file if present (silently ignore if missing, fine for production)
	_ = godotenv.Load()

	jwtSecret := os.Getenv("JWT_SECRET")
	if jwtSecret == "" {
		log.Fatal("JWT_SECRET environment variable is required")
	}
	dbURL := os.Getenv("DATABASE_URL")
	if dbURL == "" {
		dbURL = "postgres://bookmark:localdev@localhost:5432/bookmarks?sslmode=disable"
	}

	config, err := pgxpool.ParseConfig(dbURL)
	if err != nil {
		log.Fatalf("failed to parse DB URL: %v", err)
	}
	config.MaxConns = 20
	config.MinConns = 5
	pool, err := pgxpool.NewWithConfig(context.Background(), config)

	if err != nil {
		log.Fatalf("failed to create connection pool: %v", err)
	}
	defer pool.Close()

	if err := pool.Ping(context.Background()); err != nil {
		log.Fatalf("failed to ping database: %v", err)
	}
	log.Println("connected to postgres")

	store := &bookmarkStore{pool: pool}
	users := &userStore{pool: pool}

	r := chi.NewRouter()
	r.Use(middleware.Logger)
	r.Use(middleware.Recoverer)
	r.Use(metricsMiddleware)

	r.Get("/", func(w http.ResponseWriter, req *http.Request) {
		w.Write([]byte("Bookmark service is running (deployed via CI/CD)\n"))
	})

	r.Get("/health", func(w http.ResponseWriter, req *http.Request) {
		// Quick DB check — should be < 100ms
		ctx, cancel := context.WithTimeout(req.Context(), 2*time.Second)
		defer cancel()

		if err := pool.Ping(ctx); err != nil {
			log.Printf("health check: DB ping failed: %v", err)
			http.Error(w, "database unavailable", http.StatusServiceUnavailable)
			return
		}

		w.Write([]byte("ok\n"))
	})

	r.Handle("/metrics", promhttp.Handler())

	r.Post("/signup", func(w http.ResponseWriter, req *http.Request) {
		var body signupRequest
		if err := json.NewDecoder(req.Body).Decode(&body); err != nil {
			http.Error(w, "invalid JSON: "+err.Error(), http.StatusBadRequest)
			return
		}
		if body.Email == "" {
			http.Error(w, "email is required", http.StatusBadRequest)
			return
		}
		if len(body.Password) < 8 {
			http.Error(w, "password must be at least 8 characters", http.StatusBadRequest)
			return
		}

		hash, err := bcrypt.GenerateFromPassword([]byte(body.Password), 8)
		if err != nil {
			log.Printf("bcrypt error: %v", err)
			http.Error(w, "internal server error", http.StatusInternalServerError)
			return
		}

		user, err := users.Create(req.Context(), body.Email, string(hash))
		if err != nil {
			// could be "duplicate email" — check the error
			log.Printf("failed to create user: %v", err)
			http.Error(w, "could not create user (email may already be taken)", http.StatusBadRequest)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusCreated)
		json.NewEncoder(w).Encode(user)
	})

	r.Post("/login", func(w http.ResponseWriter, req *http.Request) {
		var body loginRequest
		if err := json.NewDecoder(req.Body).Decode(&body); err != nil {
			http.Error(w, "invalid JSON: "+err.Error(), http.StatusBadRequest)
			return
		}
		if body.Email == "" || body.Password == "" {
			http.Error(w, "email and password are required", http.StatusBadRequest)
			return
		}

		// Look up the user by email
		user, err := users.FindByEmail(req.Context(), body.Email)
		if err != nil {
			// Whether user not found or DB error, return the same generic message.
			// Don't leak whether the email exists or not.
			log.Printf("login failed (find): %v", err)
			http.Error(w, "invalid email or password", http.StatusUnauthorized)
			return
		}

		// Verify the password against the stored hash
		if err := bcrypt.CompareHashAndPassword([]byte(user.PasswordHash), []byte(body.Password)); err != nil {
			log.Printf("login failed (bcrypt): %v", err)
			http.Error(w, "invalid email or password", http.StatusUnauthorized)
			return
		}

		// Generate a JWT
		claims := jwt.MapClaims{
			"user_id": user.ID,
			"exp":     time.Now().Add(24 * time.Hour).Unix(),
		}
		token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
		tokenString, err := token.SignedString([]byte(jwtSecret))
		if err != nil {
			log.Printf("token signing failed: %v", err)
			http.Error(w, "internal server error", http.StatusInternalServerError)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(loginResponse{Token: tokenString})
	})

	// Protected routes — wrap in a group with auth middleware
	r.Group(func(r chi.Router) {
		r.Use(authMiddleware(jwtSecret))

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

			userID, ok := userIDFromContext(req.Context())
			if !ok {
				log.Printf("missing user_id in context — middleware bug")
				http.Error(w, "internal server error", http.StatusInternalServerError)
				return
			}

			bookmark, err := store.Add(req.Context(), userID, body.URL, body.Title)
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
			userID, ok := userIDFromContext(req.Context())
			if !ok {
				log.Printf("missing user_id in context — middleware bug")
				http.Error(w, "internal server error", http.StatusInternalServerError)
				return
			}
			bookmarks, err := store.List(req.Context(), userID)
			if err != nil {
				log.Printf("failed to list bookmarks: %v", err)
				http.Error(w, "internal server error", http.StatusInternalServerError)
				return
			}

			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(bookmarks)
		})
	})

	addr := ":8080"
	srv := &http.Server{
		Addr:         addr,
		Handler:      r,
		ReadTimeout:  10 * time.Second,
		WriteTimeout: 30 * time.Second,
		IdleTimeout:  60 * time.Second,
	}

	// Run server in a goroutine so it doesn't block
	go func() {
		log.Printf("Listening on %s", addr)
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("server failed: %v", err)
		}
	}()

	// Wait for interrupt signal (SIGINT or SIGTERM)
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit
	log.Println("shutdown signal received, draining requests...")

	// Give in-flight requests up to 15 seconds to complete
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	if err := srv.Shutdown(ctx); err != nil {
		log.Fatalf("server forced to shutdown: %v", err)
	}
	log.Println("server stopped cleanly")
}
