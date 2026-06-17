package handler

import (
	// Standard packages
	"strings" // For string manipulation

	// External packages
	"github.com/gofiber/fiber/v2" // For Fiber HTTP framework
)

// fullName function to build a display name from first and last name
func fullName(firstName, lastName string) string {
	return strings.TrimSpace(firstName + " " + lastName)
}

// userResponse function to build the standard user JSON object for API responses
func userResponse(id, email, firstName, lastName, role string) fiber.Map {
	return fiber.Map{
		"id":        id,
		"email":     email,
		"firstName": firstName,
		"lastName":  lastName,
		"name":      fullName(firstName, lastName),
		"role":      role,
	}
}
