package utils

import (
	"github.com/gofiber/fiber/v2"
)

// APIResponse is the standard JSON response structure for all endpoints.
type APIResponse struct {
	Success bool        `json:"success"`
	Data    interface{} `json:"data,omitempty"`
	Error   *APIError   `json:"error,omitempty"`
	Meta    *Meta       `json:"meta,omitempty"`
}

// APIError represents an error in the API response.
type APIError struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}

// Meta holds pagination metadata.
type Meta struct {
	Page       int   `json:"page"`
	PerPage    int   `json:"per_page"`
	Total      int64 `json:"total"`
	TotalPages int   `json:"total_pages"`
}

// Success sends a successful JSON response.
func Success(c *fiber.Ctx, data interface{}) error {
	return c.JSON(APIResponse{
		Success: true,
		Data:    data,
	})
}

// SuccessWithMeta sends a successful JSON response with pagination metadata.
func SuccessWithMeta(c *fiber.Ctx, data interface{}, meta *Meta) error {
	return c.JSON(APIResponse{
		Success: true,
		Data:    data,
		Meta:    meta,
	})
}

// SuccessCreated sends a 201 Created response.
func SuccessCreated(c *fiber.Ctx, data interface{}) error {
	return c.Status(fiber.StatusCreated).JSON(APIResponse{
		Success: true,
		Data:    data,
	})
}

// SuccessAccepted sends a 202 Accepted response.
func SuccessAccepted(c *fiber.Ctx, data interface{}) error {
	return c.Status(fiber.StatusAccepted).JSON(APIResponse{
		Success: true,
		Data:    data,
	})
}

// SuccessNoContent sends a 204 No Content response.
func SuccessNoContent(c *fiber.Ctx) error {
	return c.SendStatus(fiber.StatusNoContent)
}

// Error sends an error JSON response with the given status code and message.
func Error(c *fiber.Ctx, status int, message string) error {
	return c.Status(status).JSON(APIResponse{
		Success: false,
		Error: &APIError{
			Code:    status,
			Message: message,
		},
	})
}

// BadRequest sends a 400 error response.
func BadRequest(c *fiber.Ctx, message string) error {
	return Error(c, fiber.StatusBadRequest, message)
}

// Unauthorized sends a 401 error response.
func Unauthorized(c *fiber.Ctx, message string) error {
	return Error(c, fiber.StatusUnauthorized, message)
}

// Forbidden sends a 403 error response.
func Forbidden(c *fiber.Ctx, message string) error {
	return Error(c, fiber.StatusForbidden, message)
}

// NotFound sends a 404 error response.
func NotFound(c *fiber.Ctx, message string) error {
	return Error(c, fiber.StatusNotFound, message)
}

// Conflict sends a 409 error response.
func Conflict(c *fiber.Ctx, message string) error {
	return Error(c, fiber.StatusConflict, message)
}

// UnprocessableEntity sends a 422 error response.
func UnprocessableEntity(c *fiber.Ctx, message string) error {
	return Error(c, fiber.StatusUnprocessableEntity, message)
}

// TooManyRequests sends a 429 error response.
func TooManyRequests(c *fiber.Ctx, message string) error {
	return Error(c, fiber.StatusTooManyRequests, message)
}

// InternalError sends a 500 error response.
func InternalError(c *fiber.Ctx, message string) error {
	return Error(c, fiber.StatusInternalServerError, message)
}
