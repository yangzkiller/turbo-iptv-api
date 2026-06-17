package auth

import (
	// Standard packages
	"strings" // For string manipulation
	"time" // For time management

	// External packages
	"github.com/gofiber/fiber/v2" // For HTTP framework
	"github.com/golang-jwt/jwt/v5" // For JWT authentication
	"golang.org/x/crypto/bcrypt" // For password hashing
)

const (
	RoleAdmin = "admin"
	RoleUser  = "user"
)

// Claims struct for the JWT token
type Claims struct {
	UserID string `json:"userId"`
	Email  string `json:"email"`
	Role   string `json:"role"`
	jwt.RegisteredClaims
}

// Service struct for the authentication service
type Service struct {
	secret []byte
}

// NewService function to create a new authentication service
func NewService(secret string) *Service {
	return &Service{secret: []byte(secret)}
}

// HashPassword function to hash a password
func (s *Service) HashPassword(password string) (string, error) {
	bytes, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	return string(bytes), err
}

// CheckPassword function to check if a password is correct
func (s *Service) CheckPassword(hash, password string) bool {
	return bcrypt.CompareHashAndPassword([]byte(hash), []byte(password)) == nil
}

// CreateToken function to create a new JWT token
func (s *Service) CreateToken(userID, email, role string) (string, error) {
	claims := Claims{
		UserID: userID,
		Email:  email,
		Role:   role,
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(7 * 24 * time.Hour)),
			IssuedAt:  jwt.NewNumericDate(time.Now()),
		},
	}

	// Create a new JWT token with the claims
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return token.SignedString(s.secret)
}

// ParseToken function to parse a JWT token
func (s *Service) ParseToken(tokenStr string) (*Claims, error) {
	token, err := jwt.ParseWithClaims(tokenStr, &Claims{}, func(t *jwt.Token) (interface{}, error) {
		return s.secret, nil
	})
	if err != nil {
		return nil, err
	}

	claims, ok := token.Claims.(*Claims)
	if !ok || !token.Valid {
		return nil, fiber.ErrUnauthorized
	}

	return claims, nil
}

// Middleware function to authenticate a request
func (s *Service) Middleware() fiber.Handler {
	return func(c *fiber.Ctx) error {
		header := c.Get("Authorization")
		if !strings.HasPrefix(header, "Bearer ") {
			return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{"error": "Não autorizado"})
		}

		claims, err := s.ParseToken(strings.TrimPrefix(header, "Bearer "))
		if err != nil {
			return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{"error": "Token inválido ou expirado"})
		}

		c.Locals("userID", claims.UserID)
		c.Locals("userEmail", claims.Email)
		c.Locals("userRole", claims.Role)
		return c.Next()
	}
}

// RequireAdmin middleware restricts access to admin users.
func (s *Service) RequireAdmin() fiber.Handler {
	return func(c *fiber.Ctx) error {
		role, _ := c.Locals("userRole").(string)
		if role != RoleAdmin {
			return c.Status(fiber.StatusForbidden).JSON(fiber.Map{"error": "Acesso restrito a administradores"})
		}
		return c.Next()
	}
}

// UserID function to get the user ID from the context
func UserID(c *fiber.Ctx) string {
	id, _ := c.Locals("userID").(string)
	return id
}

// UserRole function to get the user role from the context
func UserRole(c *fiber.Ctx) string {
	role, _ := c.Locals("userRole").(string)
	return role
}
