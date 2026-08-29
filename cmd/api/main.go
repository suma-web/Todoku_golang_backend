package main

import (
	"context"
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
	"twitter_golang_backend/internal/notification"
	"twitter_golang_backend/internal/post"
	"twitter_golang_backend/internal/question"
	"twitter_golang_backend/internal/schooladmin"
	"twitter_golang_backend/internal/schoolgroup"
	"twitter_golang_backend/internal/schoolpost"
	"twitter_golang_backend/internal/search"
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

	if err := database.Migrate(context.Background(), db, "migrations"); err != nil {
		log.Fatal(err)
	}

	userRepository := user.NewRepository(db)
	userHandler := user.NewHandler(userRepository, cfg.SessionSecret)
	postRepository := post.NewRepository(db)
	postHandler := post.NewHandler(postRepository, "uploads")
	commentRepository := comment.NewRepository(db)
	commentHandler := comment.NewHandler(commentRepository)
	notificationRepository := notification.NewRepository(db)
	notificationHandler := notification.NewHandler(notificationRepository)
	schoolGroupHandler := schoolgroup.NewHandler(db)
	schoolPostRepository := schoolpost.NewRepository(db)
	schoolPostService := schoolpost.NewService(schoolPostRepository)
	schoolPostHandler := schoolpost.NewHandler(schoolPostService)
	questionRepository := question.NewRepository(db)
	questionService := question.NewService(questionRepository)
	questionHandler := question.NewHandler(questionService)
	searchRepository := search.NewRepository(db)
	searchService := search.NewService(searchRepository)
	searchHandler := search.NewHandler(searchService)
	schoolAdminHandler := schooladmin.NewHandler(db)

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

	router.Post("/api/login", userHandler.Login)
	router.Post("/api/logout", userHandler.Logout)
	router.With(auth.RequireAuth(cfg.SessionSecret)).Get("/api/me", userHandler.Me)
	router.With(auth.RequireAuth(cfg.SessionSecret), auth.RequireRole(db, "admin")).Post("/api/admin/users", userHandler.AdminCreateUser)
	router.With(auth.RequireAuth(cfg.SessionSecret)).Get("/api/school-groups", schoolGroupHandler.List)
	router.With(auth.RequireAuth(cfg.SessionSecret)).Get("/api/users/{userId}/school-groups", schoolGroupHandler.UserGroups)
	router.With(auth.RequireAuth(cfg.SessionSecret), auth.RequireRole(db, "admin")).Post("/api/school-groups", schoolGroupHandler.Create)
	router.With(auth.RequireAuth(cfg.SessionSecret), auth.RequireRole(db, "admin")).Delete("/api/school-groups/{groupId}", schoolGroupHandler.Delete)
	router.With(auth.RequireAuth(cfg.SessionSecret), auth.RequireRole(db, "admin")).Post("/api/school-groups/{groupId}/members", schoolGroupHandler.AddMember)
	router.With(auth.RequireAuth(cfg.SessionSecret), auth.RequireRole(db, "admin")).Delete("/api/school-groups/{groupId}/members/{userId}", schoolGroupHandler.RemoveMember)
	router.With(auth.RequireAuth(cfg.SessionSecret), auth.RequireRole(db, "teacher", "admin")).Post("/api/school-posts", schoolPostHandler.Create)
	router.With(auth.RequireAuth(cfg.SessionSecret)).Get("/api/school-posts/{id}", schoolPostHandler.Get)
	router.With(auth.RequireAuth(cfg.SessionSecret)).Delete("/api/school-posts/{id}", schoolPostHandler.Delete)
	router.With(auth.RequireAuth(cfg.SessionSecret)).Get("/api/timeline", schoolPostHandler.Timeline)
	router.With(auth.RequireAuth(cfg.SessionSecret)).Post("/api/school-posts/{id}/read", schoolPostHandler.Read)
	router.With(auth.RequireAuth(cfg.SessionSecret)).Post("/api/school-posts/{id}/confirm", schoolPostHandler.Confirm)
	router.With(auth.RequireAuth(cfg.SessionSecret), auth.RequireRole(db, "teacher", "admin")).Get("/api/school-posts/{id}/status", schoolPostHandler.Status)
	router.With(auth.RequireAuth(cfg.SessionSecret), auth.RequireRole(db, "teacher", "admin")).Get("/api/school-posts/{id}/unconfirmed", schoolPostHandler.Unconfirmed)
	router.With(auth.RequireAuth(cfg.SessionSecret)).Get("/api/question-categories", questionHandler.ListCategories)
	router.With(auth.RequireAuth(cfg.SessionSecret), auth.RequireRole(db, "admin")).Post("/api/question-categories", questionHandler.CreateCategory)
	router.With(auth.RequireAuth(cfg.SessionSecret)).Post("/api/questions", questionHandler.Create)
	router.With(auth.RequireAuth(cfg.SessionSecret)).Get("/api/questions", questionHandler.List)
	router.With(auth.RequireAuth(cfg.SessionSecret)).Get("/api/questions/{id}", questionHandler.Get)
	router.With(auth.RequireAuth(cfg.SessionSecret)).Get("/api/questions/{id}/answers", questionHandler.ListAnswers)
	router.With(auth.RequireAuth(cfg.SessionSecret)).Post("/api/questions/{id}/answers", questionHandler.Answer)
	router.With(auth.RequireAuth(cfg.SessionSecret)).Patch("/api/questions/{id}/resolve", questionHandler.Resolve)
	router.With(auth.RequireAuth(cfg.SessionSecret)).Get("/api/search", searchHandler.Search)
	router.With(auth.RequireAuth(cfg.SessionSecret), auth.RequireRole(db, "admin")).Get("/api/admin/users", schoolAdminHandler.ListUsers)
	router.With(auth.RequireAuth(cfg.SessionSecret), auth.RequireRole(db, "admin")).Patch("/api/admin/users/{id}", schoolAdminHandler.UpdateUser)
	router.With(auth.RequireAuth(cfg.SessionSecret), auth.RequireRole(db, "admin")).Get("/api/school-groups/{groupId}/members", schoolGroupHandler.Members)
	router.With(auth.RequireAuth(cfg.SessionSecret)).Patch("/api/me", userHandler.UpdateProfile)
	router.With(auth.RequireAuth(cfg.SessionSecret)).Delete("/api/me", userHandler.DeleteAccount)
	router.With(auth.RequireAuth(cfg.SessionSecret)).Get("/api/users/{name}", userHandler.Profile)
	router.With(auth.RequireAuth(cfg.SessionSecret)).Get("/api/posts", postHandler.List)
	router.With(auth.RequireAuth(cfg.SessionSecret)).Get("/api/posts/{id}", postHandler.Get)
	router.With(auth.RequireAuth(cfg.SessionSecret)).Delete("/api/posts/{id}", postHandler.Delete)
	router.With(auth.RequireAuth(cfg.SessionSecret)).Post("/api/posts", postHandler.Create)
	router.With(auth.RequireAuth(cfg.SessionSecret)).Post("/api/posts/{id}/bookmarks", postHandler.Bookmark)
	router.With(auth.RequireAuth(cfg.SessionSecret)).Delete("/api/posts/{id}/bookmarks", postHandler.UndoBookmark)
	router.With(auth.RequireAuth(cfg.SessionSecret)).Get("/api/bookmarks", postHandler.ListBookmarks)
	router.With(auth.RequireAuth(cfg.SessionSecret)).Get("/api/me/comments", commentHandler.ListMine)
	router.With(auth.RequireAuth(cfg.SessionSecret)).Get("/api/posts/{id}/comments", commentHandler.List)
	router.With(auth.RequireAuth(cfg.SessionSecret)).Post("/api/posts/{id}/comments", commentHandler.Create)
	router.With(auth.RequireAuth(cfg.SessionSecret)).Delete("/api/comments/{id}", commentHandler.Delete)
	router.With(auth.RequireAuth(cfg.SessionSecret)).Get("/api/notifications", notificationHandler.List)
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
