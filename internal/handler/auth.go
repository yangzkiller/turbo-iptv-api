package handler

import (
	// Standard packages
	"context" // For context management
	"strings" // For string manipulation

	// External packages
	"github.com/gofiber/fiber/v2" // For Fiber HTTP framework

	// Internal packages
	"turbo-iptv-api/internal/auth" // For authentication service
)

// createUserRequest struct for the JSON body of user creation endpoints
type createUserRequest struct {
	Email     string `json:"email"`     // User e-mail address
	Password  string `json:"password"`  // Plain text password (hashed before storage)
	FirstName string `json:"firstName"` // User first name
	LastName  string `json:"lastName"`  // User last name
	Name      string `json:"name"`      // Optional full name (split into first/last when names are empty)
	Role      string `json:"role"`      // Optional role: admin or user (admin route only)
}

// createUserData struct for validated and normalized user data ready for persistence
type createUserData struct {
	Email     string // Normalized e-mail (lowercase, trimmed)
	Password  string // Plain text password (hashed at insert time)
	FirstName string // Trimmed first name
	LastName  string // Trimmed last name
}

// loginRequest struct for the JSON body of the login endpoint
type loginRequest struct {
	Email    string `json:"email"`    // User e-mail address
	Password string `json:"password"` // Plain text password for verification
}

// Register function to handle public sign-up (always creates role user and returns a JWT)
func (h *Handler) Register(c *fiber.Ctx) error {
	return h.createUserFromRequest(c, auth.RoleUser, true)
}

// CreateUser function to let an admin create admin or user accounts (no JWT returned)
func (h *Handler) CreateUser(c *fiber.Ctx) error {
	return h.createUserFromRequest(c, "", false)
}

// createUserFromRequest function to shared user creation flow for public and admin routes
func (h *Handler) createUserFromRequest(c *fiber.Ctx, forcedRole string, withToken bool) error {
	// Parse JSON body into request struct
	var req createUserRequest
	if err := c.BodyParser(&req); err != nil {
		return c.Status(400).JSON(fiber.Map{"error": "Dados inválidos"})
	}

	// Validate and normalize input fields
	data, errMsg := validateCreateUserRequest(req)
	if errMsg != "" {
		return c.Status(400).JSON(fiber.Map{"error": errMsg})
	}

	// Use forced role on public route; parse role from body on admin route
	role := forcedRole
	if role == "" {
		var roleErr string
		role, roleErr = parseRole(req.Role)
		if roleErr != "" {
			return c.Status(400).JSON(fiber.Map{"error": roleErr})
		}
	}

	// Persist user in the database
	userID, err := h.insertUser(c.Context(), data, role)
	if err != nil {
		return respondCreateUserError(c, err)
	}

	// Build response with user data
	response := fiber.Map{
		"user": userResponse(userID, data.Email, data.FirstName, data.LastName, role),
	}

	// Include JWT on public register so the client is logged in immediately
	if withToken {
		token, err := h.Auth.CreateToken(userID, data.Email, role)
		if err != nil {
			return c.Status(500).JSON(fiber.Map{"error": "Erro ao gerar token"})
		}
		response["token"] = token
	}

	return c.Status(201).JSON(response)
}

// Login function to authenticate a user and return a JWT
func (h *Handler) Login(c *fiber.Ctx) error {
	// Parse JSON body into request struct
	var body loginRequest
	if err := c.BodyParser(&body); err != nil {
		return c.Status(400).JSON(fiber.Map{"error": "Dados inválidos"})
	}

	// Normalize and validate required fields
	body.Email = strings.TrimSpace(strings.ToLower(body.Email))
	if body.Email == "" || body.Password == "" {
		return c.Status(400).JSON(fiber.Map{"error": "E-mail e senha são obrigatórios"})
	}

	// Fetch user credentials from the database
	var userID, hash, firstName, lastName, role string
	err := h.DB.QueryRow(c.Context(), `
		SELECT id::text, password, first_name, last_name, role
		FROM users WHERE email = $1
	`, body.Email).Scan(&userID, &hash, &firstName, &lastName, &role)
	if err != nil || !h.Auth.CheckPassword(hash, body.Password) {
		return c.Status(401).JSON(fiber.Map{"error": "E-mail ou senha incorretos"})
	}

	// Generate JWT for the authenticated session
	token, err := h.Auth.CreateToken(userID, body.Email, role)
	if err != nil {
		return c.Status(500).JSON(fiber.Map{"error": "Erro ao gerar token"})
	}

	return c.JSON(fiber.Map{
		"token": token,
		"user":  userResponse(userID, body.Email, firstName, lastName, role),
	})
}

// Me function to return the profile of the authenticated user
func (h *Handler) Me(c *fiber.Ctx) error {
	// Get user ID injected by auth middleware
	userID := auth.UserID(c)

	// Fetch profile from the database
	var email, firstName, lastName, role string
	err := h.DB.QueryRow(c.Context(), `
		SELECT email, first_name, last_name, role FROM users WHERE id = $1
	`, userID).Scan(&email, &firstName, &lastName, &role)
	if err != nil {
		return c.Status(404).JSON(fiber.Map{"error": "Usuário não encontrado"})
	}

	return c.JSON(userResponse(userID, email, firstName, lastName, role))
}

// BootstrapAdmin function to create the first admin from environment variables on startup
func (h *Handler) BootstrapAdmin(ctx context.Context, email, password, firstName, lastName string) error {
	email = strings.TrimSpace(strings.ToLower(email))
	// Skip bootstrap when env vars are not configured
	if email == "" || password == "" {
		return nil
	}

	// Only bootstrap when no admin exists yet
	var count int
	if err := h.DB.QueryRow(ctx, `SELECT COUNT(*) FROM users WHERE role = $1`, auth.RoleAdmin).Scan(&count); err != nil {
		return err
	}
	if count > 0 {
		return nil
	}

	// Apply default display name when not provided
	if firstName == "" {
		firstName = "Admin"
	}
	if lastName == "" {
		lastName = "Sistema"
	}

	data := &createUserData{
		Email:     email,
		Password:  password,
		FirstName: firstName,
		LastName:  lastName,
	}

	_, err := h.insertUser(ctx, data, auth.RoleAdmin)
	return err
}

// validateCreateUserRequest function to normalize and validate user creation input
func validateCreateUserRequest(req createUserRequest) (*createUserData, string) {
	// Normalize string fields
	req.Email = strings.TrimSpace(strings.ToLower(req.Email))
	req.FirstName = strings.TrimSpace(req.FirstName)
	req.LastName = strings.TrimSpace(req.LastName)

	// Split full name when first/last were not sent separately
	if req.FirstName == "" && req.Name != "" {
		parts := strings.SplitN(strings.TrimSpace(req.Name), " ", 2)
		req.FirstName = parts[0]
		if len(parts) > 1 {
			req.LastName = parts[1]
		}
	}

	// Validate required fields
	if req.Email == "" || req.Password == "" || req.FirstName == "" || req.LastName == "" {
		return nil, "Nome, sobrenome, e-mail e senha são obrigatórios"
	}
	if len(req.Password) < 6 {
		return nil, "Senha deve ter pelo menos 6 caracteres"
	}

	return &createUserData{
		Email:     req.Email,
		Password:  req.Password,
		FirstName: req.FirstName,
		LastName:  req.LastName,
	}, ""
}

// parseRole function to normalize and validate the role field from the request body
func parseRole(raw string) (string, string) {
	role := strings.ToLower(strings.TrimSpace(raw))
	// Default to user when role is omitted
	if role == "" {
		return auth.RoleUser, ""
	}
	if role != auth.RoleAdmin && role != auth.RoleUser {
		return "", "Role deve ser admin ou user"
	}
	return role, ""
}

// insertUser function to hash the password and insert a new user into the database
func (h *Handler) insertUser(ctx context.Context, data *createUserData, role string) (string, error) {
	// Hash password before storing
	hash, err := h.Auth.HashPassword(data.Password)
	if err != nil {
		return "", err
	}

	// Insert user and return generated UUID
	var userID string
	err = h.DB.QueryRow(ctx, `
		INSERT INTO users (first_name, last_name, email, password, role)
		VALUES ($1, $2, $3, $4, $5)
		RETURNING id::text
	`, data.FirstName, data.LastName, data.Email, hash, role).Scan(&userID)
	return userID, err
}

// respondCreateUserError function to map database errors to HTTP responses on user creation
func respondCreateUserError(c *fiber.Ctx, err error) error {
	if strings.Contains(err.Error(), "duplicate") || strings.Contains(err.Error(), "unique") {
		return c.Status(409).JSON(fiber.Map{"error": "E-mail já cadastrado"})
	}
	return c.Status(500).JSON(fiber.Map{"error": "Erro ao criar conta"})
}
