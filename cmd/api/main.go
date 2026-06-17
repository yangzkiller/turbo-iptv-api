package main

import (
	// Standard packages
	"context" // For context management
	"fmt" // For formatting strings
	"log" // For logging
	"time" // For time management

	// External packages
	"github.com/gofiber/fiber/v2" // For HTTP framework
	"github.com/gofiber/fiber/v2/middleware/cors" // For CORS middleware

	// Internal packages
	"turbo-iptv-api/internal/auth" // For authentication
	"turbo-iptv-api/internal/config" // For configuration
	"turbo-iptv-api/internal/db" // For database connection
	"turbo-iptv-api/internal/handler" // For handler functions
	"turbo-iptv-api/internal/playlist" // For playlist functions
)

// Main function for the API, it loads the configuration, connects to the database, and starts the server.
func main() {
	// Load the configuration
	cfg := config.Load()

	// Create a context with a timeout of 30 seconds
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	// Connect to the PostgreSQL database
	pool, err := db.Connect(ctx, cfg.DatabaseURL)
	if err != nil {
		log.Fatal(err)
	}
	// Close the database connection pool when the application exits
	defer pool.Close()
	
	// Create a new authentication service
	authService := auth.NewService(cfg.JWTSecret)
	h := &handler.Handler{
		DB:    pool,
		Auth:  authService,
		Store: playlist.NewStore(),
	}

	if err := h.BootstrapAdmin(ctx, cfg.BootstrapAdminEmail, cfg.BootstrapAdminPassword, cfg.BootstrapAdminFirstName, cfg.BootstrapAdminLastName); err != nil {
		log.Printf("aviso: falha ao criar admin inicial: %v", err)
	}

	// Create a new Fiber app
	app := fiber.New()

	// Use the CORS middleware
	app.Use(cors.New(cors.Config{
		AllowOrigins: cfg.CORSOrigins,
		AllowHeaders: "Origin, Content-Type, Accept, Authorization",
	}))

	// Get the root route
	app.Get("/", func(c *fiber.Ctx) error {
		return c.JSON(fiber.Map{
			"status": "ok",
			"routes": fiber.Map{
				"POST /auth/register":                          "Criar conta (sempre role user)",
				"POST /auth/login":                             "Entrar",
				"POST /admin/users":                            "Admin: criar conta admin ou user",
				"GET /me/playlists":                            "Listar playlists salvas",
				"POST /me/playlists/:id/connect":               "Conectar playlist salva",
				"GET /me/playlists/:id/favorites":              "Favoritos",
				"GET /me/playlists/:id/progress":               "Continuar assistindo",
				"POST /connect-xc":                             "Conectar via Xtream Codes",
				"GET /connect-m3u?url=&type=live|movie|series": "Conectar via playlist M3U",
				"GET /playlist/:sessionId/channels?type=":      "Listar conteúdo",
				"GET /playlist/:sessionId/categories?type=":    "Listar categorias",
			},
		})
	})

	authRoutes := app.Group("/auth")
	authRoutes.Post("/register", h.Register)
	authRoutes.Post("/login", h.Login)

	admin := app.Group("/admin", authService.Middleware(), authService.RequireAdmin())
	admin.Post("/users", h.CreateUser)

	me := app.Group("/me", authService.Middleware())
	me.Get("/", h.Me)
	me.Get("/playlists", h.ListPlaylists)
	me.Post("/playlists", h.CreatePlaylist)
	me.Patch("/playlists/:id", h.UpdatePlaylist)
	me.Delete("/playlists/:id", h.DeletePlaylist)
	me.Post("/playlists/:id/connect", h.ConnectSavedPlaylist)

	me.Get("/playlists/:playlistId/favorites", h.ListFavorites)
	me.Post("/playlists/:playlistId/favorites", h.AddFavorite)
	me.Delete("/playlists/:playlistId/favorites/:id", h.DeleteFavorite)
	me.Delete("/playlists/:playlistId/favorites/key/:contentKey", h.DeleteFavoriteByKey)

	me.Get("/playlists/:playlistId/progress", h.ListProgress)
	me.Get("/playlists/:playlistId/progress/:contentKey", h.GetProgress)
	me.Put("/playlists/:playlistId/progress", h.UpsertProgress)
	me.Delete("/playlists/:playlistId/progress/:contentKey", h.DeleteProgress)

	app.Post("/connect-xc", h.ConnectXC)
	app.Get("/connect-m3u", h.ConnectM3U)
	app.Get("/playlist/:sessionId/channels", h.SessionChannels)
	app.Get("/playlist/:sessionId/categories", h.SessionCategories)

	addr := fmt.Sprintf(":%s", cfg.Port)
	log.Printf("API ouvindo em %s", addr)
	log.Fatal(app.Listen(addr))
}
