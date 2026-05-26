package handler

import (
	"github.com/gofiber/fiber/v2"
	"github.com/google/uuid"
	"github.com/realsend/be-realsend/internal/repository"
	"github.com/realsend/be-realsend/internal/service"
	"github.com/realsend/be-realsend/internal/utils"
)

type AuthHandler struct {
	authService service.AuthService
	auditRepo   repository.AuditLogRepository
}

func NewAuthHandler(authService service.AuthService, auditRepo repository.AuditLogRepository) *AuthHandler {
	return &AuthHandler{
		authService: authService,
		auditRepo:   auditRepo,
	}
}

type registerRequest struct {
	Email    string `json:"email" validate:"required,email"`
	Password string `json:"password" validate:"required,min=8,max=72"`
	FullName string `json:"full_name" validate:"required,min=2,max=100"`
}

type loginRequest struct {
	Email    string `json:"email" validate:"required,email"`
	Password string `json:"password" validate:"required"`
}

type updateProfileRequest struct {
	FullName    string `json:"full_name" validate:"required,min=2,max=100"`
	CompanyName string `json:"company_name" validate:"max=100"`
	Email       string `json:"email" validate:"required,email"`
}

type changePasswordRequest struct {
	OldPassword string `json:"old_password" validate:"required"`
	NewPassword string `json:"new_password" validate:"required,min=8,max=72"`
}

// Register handles user registration.
func (h *AuthHandler) Register(c *fiber.Ctx) error {
	var req registerRequest
	if err := c.BodyParser(&req); err != nil {
		return utils.BadRequest(c, "invalid request body")
	}

	if err := utils.ValidateStruct(&req); err != nil {
		return utils.UnprocessableEntity(c, err.Error())
	}

	user, err := h.authService.Register(c.Context(), req.Email, req.Password, req.FullName)
	if err != nil {
		return utils.Conflict(c, err.Error())
	}

	// Audit log
	utils.LogAction(c.Context(), h.auditRepo, c, user.ID, "auth.register", "user", &user.ID, map[string]string{"email": user.Email})

	return utils.SuccessCreated(c, user)
}

// Login handles user authentication.
func (h *AuthHandler) Login(c *fiber.Ctx) error {
	var req loginRequest
	if err := c.BodyParser(&req); err != nil {
		return utils.BadRequest(c, "invalid request body")
	}

	if err := utils.ValidateStruct(&req); err != nil {
		return utils.UnprocessableEntity(c, err.Error())
	}

	token, user, err := h.authService.Login(c.Context(), req.Email, req.Password)
	if err != nil {
		return utils.Unauthorized(c, err.Error())
	}

	// Audit log
	utils.LogAction(c.Context(), h.auditRepo, c, user.ID, "auth.login", "user", &user.ID, map[string]string{"email": user.Email})

	return utils.Success(c, fiber.Map{
		"token": token,
		"user":  user,
	})
}

// Me returns the profile of the currently logged-in user.
func (h *AuthHandler) Me(c *fiber.Ctx) error {
	userIDStr, ok := c.Locals("user_id").(string)
	if !ok {
		return utils.Unauthorized(c, "unauthorized")
	}

	userID, err := uuid.Parse(userIDStr)
	if err != nil {
		return utils.BadRequest(c, "invalid user id")
	}

	user, err := h.authService.GetProfile(c.Context(), userID)
	if err != nil {
		return utils.NotFound(c, err.Error())
	}

	return utils.Success(c, user)
}

// UpdateProfile handles profile modifications.
func (h *AuthHandler) UpdateProfile(c *fiber.Ctx) error {
	userIDStr, ok := c.Locals("user_id").(string)
	if !ok {
		return utils.Unauthorized(c, "unauthorized")
	}

	userID, err := uuid.Parse(userIDStr)
	if err != nil {
		return utils.BadRequest(c, "invalid user id")
	}

	var req updateProfileRequest
	if err := c.BodyParser(&req); err != nil {
		return utils.BadRequest(c, "invalid request body")
	}

	if err := utils.ValidateStruct(&req); err != nil {
		return utils.UnprocessableEntity(c, err.Error())
	}

	user, err := h.authService.UpdateProfile(c.Context(), userID, req.FullName, req.CompanyName, req.Email)
	if err != nil {
		return utils.InternalError(c, err.Error())
	}

	// Audit log
	utils.LogAction(c.Context(), h.auditRepo, c, userID, "auth.profile_updated", "user", &userID, map[string]string{"email": user.Email, "full_name": user.FullName})

	return utils.Success(c, user)
}

// ChangePassword handles password updates.
func (h *AuthHandler) ChangePassword(c *fiber.Ctx) error {
	userIDStr, ok := c.Locals("user_id").(string)
	if !ok {
		return utils.Unauthorized(c, "unauthorized")
	}

	userID, err := uuid.Parse(userIDStr)
	if err != nil {
		return utils.BadRequest(c, "invalid user id")
	}

	var req changePasswordRequest
	if err := c.BodyParser(&req); err != nil {
		return utils.BadRequest(c, "invalid request body")
	}

	if err := utils.ValidateStruct(&req); err != nil {
		return utils.UnprocessableEntity(c, err.Error())
	}

	err = h.authService.ChangePassword(c.Context(), userID, req.OldPassword, req.NewPassword)
	if err != nil {
		return utils.BadRequest(c, err.Error())
	}

	// Audit log
	utils.LogAction(c.Context(), h.auditRepo, c, userID, "auth.password_changed", "user", &userID, nil)

	return utils.Success(c, fiber.Map{
		"message": "password updated successfully",
	})
}

// Logout handles user logout audit log.
func (h *AuthHandler) Logout(c *fiber.Ctx) error {
	userIDStr, ok := c.Locals("user_id").(string)
	if !ok {
		return utils.Unauthorized(c, "unauthorized")
	}

	userID, err := uuid.Parse(userIDStr)
	if err != nil {
		return utils.BadRequest(c, "invalid user id")
	}

	user, err := h.authService.GetProfile(c.Context(), userID)
	var email string
	if err == nil && user != nil {
		email = user.Email
	}

	// Audit log
	utils.LogAction(c.Context(), h.auditRepo, c, userID, "auth.logout", "user", &userID, map[string]string{"email": email})

	return utils.Success(c, fiber.Map{
		"message": "logged out successfully",
	})
}
