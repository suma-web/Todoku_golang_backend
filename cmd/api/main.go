package main

import (
	"log"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/go-chi/cors"

	"twitter_golang_backend/internal/auth"
	"twitter_golang_backend/internal/comment"
	"twitter_golang_backend/internal/config"
	"twitter_golang_backend/internal/database"
	"twitter_golang_backend/internal/post"
	"twitter_golang_backend/internal/user"
)

func main() {
	cfg, err := config.Load()
	if err != nil {
		log.Fatal(err)
	}

	db, err := database.Open(cfg.DatabaseURL)
	if err != nil {
		log.Fatal(err)
	}
	defer db.Close()

	userRepository := user.NewRepository(db)
	userHandler := user.NewHandler(userRepository, cfg.SessionSecret)
	postRepository := post.NewRepository(db)
	postHandler := post.NewHandler(postRepository, "uploads")
	commentRepository := comment.NewRepository(db)
	commentHandler := comment.NewHandler(commentRepository)

	router := chi.NewRouter()

	router.Use(middleware.RequestID)
	router.Use(middleware.RealIP)
	router.Use(middleware.Logger)
	router.Use(middleware.Recoverer)
	router.Use(middleware.Timeout(15 * time.Second))

	router.Use(cors.Handler(cors.Options{
		AllowedOrigins: []string{
			cfg.FrontendURL,
			"http://localhost:*",
			"http://127.0.0.1:*",
		},
		AllowedMethods: []string{
			http.MethodGet,
			http.MethodPost,
			http.MethodPatch,
			http.MethodDelete,
			http.MethodOptions,
		},
		AllowedHeaders: []string{
			"Accept",
			"Authorization",
			"Content-Type",
		},
		AllowCredentials: true,
		MaxAge:           300,
	}))

	router.Get("/health", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("ok"))
	})

	router.Post("/api/signup", userHandler.Signup)
	router.Post("/api/login", userHandler.Login)
	router.With(auth.RequireAuth(cfg.SessionSecret)).Get("/api/me", userHandler.Me)
	router.With(auth.RequireAuth(cfg.SessionSecret)).Patch("/api/me", userHandler.UpdateProfile)
	router.With(auth.RequireAuth(cfg.SessionSecret)).Get("/api/users/{name}", userHandler.Profile)
	router.With(auth.RequireAuth(cfg.SessionSecret)).Get("/api/posts", postHandler.List)
	router.With(auth.RequireAuth(cfg.SessionSecret)).Get("/api/posts/{id}", postHandler.Get)
	router.With(auth.RequireAuth(cfg.SessionSecret)).Delete("/api/posts/{id}", postHandler.Delete)
	router.With(auth.RequireAuth(cfg.SessionSecret)).Post("/api/posts", postHandler.Create)
	router.With(auth.RequireAuth(cfg.SessionSecret)).Get("/api/posts/{id}/comments", commentHandler.List)
	router.With(auth.RequireAuth(cfg.SessionSecret)).Post("/api/posts/{id}/comments", commentHandler.Create)
	router.With(auth.RequireAuth(cfg.SessionSecret)).Delete("/api/comments/{id}", commentHandler.Delete)
	router.Handle("/uploads/*", http.StripPrefix("/uploads/", http.FileServer(http.Dir("uploads"))))

	server := &http.Server{
		Addr:              ":" + cfg.Port,
		Handler:           router,
		ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout:       10 * time.Second,
		WriteTimeout:      10 * time.Second,
		IdleTimeout:       60 * time.Second,
	}

	log.Printf("server started on http://localhost:%s", cfg.Port)

	if err := server.ListenAndServe(); err != nil &&
		err != http.ErrServerClosed {
		log.Fatal(err)
	}
}
