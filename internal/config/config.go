package config

// Standard packages
import "os" // For environment variables

// Configuration struct for the API
type Config struct {
	DatabaseURL            string
	JWTSecret              string
	Port                   string
	CORSOrigins            string
	BootstrapAdminEmail    string
	BootstrapAdminPassword string
	BootstrapAdminFirstName string
	BootstrapAdminLastName  string
}

// Load function to load the configuration from the environment variables
func Load() Config {
	// Return the configuration
	return Config{
		DatabaseURL:             envOr("DATABASE_URL", "postgres://turbo:turbo@localhost:5432/turbo_iptv?sslmode=disable"),
		JWTSecret:               envOr("JWT_SECRET", "dev-secret-development"),
		Port:                    envOr("PORT", "8080"),
		CORSOrigins:             envOr("CORS_ORIGINS", "http://localhost:5173"),
		BootstrapAdminEmail:     envOr("BOOTSTRAP_ADMIN_EMAIL", ""),
		BootstrapAdminPassword:  envOr("BOOTSTRAP_ADMIN_PASSWORD", ""),
		BootstrapAdminFirstName: envOr("BOOTSTRAP_ADMIN_FIRST_NAME", "Admin"),
		BootstrapAdminLastName:  envOr("BOOTSTRAP_ADMIN_LAST_NAME", "Sistema"),
	}
}

// envOr function to get the environment variable or the fallback value
func envOr(key, fallback string) string {
	// Get the environment variable or the fallback value
	if value := os.Getenv(key); value != "" {
		return value
	}
	return fallback
}
