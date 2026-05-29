package handler

import (
	"github.com/gofiber/fiber/v2"
	"github.com/google/uuid"
	"github.com/realsend/be-realsend/internal/repository"
	"github.com/realsend/be-realsend/internal/service"
	"github.com/realsend/be-realsend/internal/utils"
)

type APIKeyHandler struct {
	keyService service.APIKeyService
	auditRepo  repository.AuditLogRepository
}

func NewAPIKeyHandler(keyService service.APIKeyService, auditRepo repository.AuditLogRepository) *APIKeyHandler {
	return &APIKeyHandler{
		keyService: keyService,
		auditRepo:  auditRepo,
	}
}

type createAPIKeyRequest struct {
	Name     string  `json:"name" validate:"required,min=2,max=100"`
	DomainID *string `json:"domain_id" validate:"omitempty,uuid"`
}

// CreateKey handles token generation and metadata insertion.
// @Summary Buat API Key baru
// @Description Membuat API Key baru untuk autentikasi pengiriman email lewat API / SMTP. Token rahasia hanya dimunculkan SATU KALI setelah dibuat.
// @Tags API Keys
// @Accept json
// @Produce json
// @Param request body createAPIKeyRequest true "Data API Key baru"
// @Success 201 {object} map[string]interface{} "API Key berhasil dibuat"
// @Failure 400 {object} map[string]interface{} "Body request tidak valid"
// @Failure 401 {object} map[string]interface{} "Unauthorized"
// @Failure 422 {object} map[string]interface{} "Validasi gagal"
// @Security BearerAuth
// @Router /api-keys [post]
func (h *APIKeyHandler) CreateKey(c *fiber.Ctx) error {
	userIDStr, ok := c.Locals("user_id").(string)
	if !ok {
		return utils.Unauthorized(c, "unauthorized")
	}

	userID, err := uuid.Parse(userIDStr)
	if err != nil {
		return utils.BadRequest(c, "invalid user id")
	}

	var req createAPIKeyRequest
	if err := c.BodyParser(&req); err != nil {
		return utils.BadRequest(c, "invalid request body")
	}

	if err := utils.ValidateStruct(&req); err != nil {
		return utils.UnprocessableEntity(c, err.Error())
	}

	var parsedDomainID *uuid.UUID
	if req.DomainID != nil && *req.DomainID != "" {
		dID, err := uuid.Parse(*req.DomainID)
		if err != nil {
			return utils.BadRequest(c, "invalid domain id format")
		}
		parsedDomainID = &dID
	}

	rawKey, keyMeta, err := h.keyService.CreateKey(c.Context(), userID, req.Name, parsedDomainID)
	if err != nil {
		return utils.Conflict(c, err.Error())
	}

	// Audit log
	utils.LogAction(c.Context(), h.auditRepo, c, userID, "api_key.created", "api_key", &keyMeta.ID, map[string]string{"name": keyMeta.Name})

	// Important: Return BOTH rawKey (the secret revealed exactly once) and stored keyMeta
	return utils.SuccessCreated(c, fiber.Map{
		"token":    rawKey,
		"metadata": keyMeta,
	})
}

// ListKeys returns all active key records for the dashboard.
// @Summary List semua API Key
// @Description Mendapatkan seluruh daftar API Key yang dimiliki oleh user saat ini.
// @Tags API Keys
// @Produce json
// @Success 200 {array} map[string]interface{} "Daftar API Key metadata"
// @Failure 401 {object} map[string]interface{} "Unauthorized"
// @Security BearerAuth
// @Router /api-keys [get]
func (h *APIKeyHandler) ListKeys(c *fiber.Ctx) error {
	userIDStr, ok := c.Locals("user_id").(string)
	if !ok {
		return utils.Unauthorized(c, "unauthorized")
	}

	userID, err := uuid.Parse(userIDStr)
	if err != nil {
		return utils.BadRequest(c, "invalid user id")
	}

	keys, err := h.keyService.ListKeys(c.Context(), userID)
	if err != nil {
		return utils.InternalError(c, err.Error())
	}

	return utils.Success(c, keys)
}

// GetKey returns a single API key metadata record.
// @Summary Detail API Key
// @Description Mendapatkan metadata API Key tertentu berdasarkan ID.
// @Tags API Keys
// @Produce json
// @Param id path string true "API Key ID UUID"
// @Success 200 {object} map[string]interface{} "Metadata API Key"
// @Failure 400 {object} map[string]interface{} "Format UUID tidak valid"
// @Failure 401 {object} map[string]interface{} "Unauthorized"
// @Failure 403 {object} map[string]interface{} "Akses dilarang"
// @Failure 404 {object} map[string]interface{} "API Key tidak ditemukan"
// @Security BearerAuth
// @Router /api-keys/{id} [get]
func (h *APIKeyHandler) GetKey(c *fiber.Ctx) error {
	userIDStr, ok := c.Locals("user_id").(string)
	if !ok {
		return utils.Unauthorized(c, "unauthorized")
	}

	userID, err := uuid.Parse(userIDStr)
	if err != nil {
		return utils.BadRequest(c, "invalid user id")
	}

	idStr := c.Params("id")
	id, err := uuid.Parse(idStr)
	if err != nil {
		return utils.BadRequest(c, "invalid api key id")
	}

	key, err := h.keyService.GetKey(c.Context(), id)
	if err != nil {
		return utils.NotFound(c, err.Error())
	}
	if key.UserID != userID {
		return utils.Forbidden(c, "you do not have permission to access this api key")
	}

	return utils.Success(c, key)
}

// RevokeKey deletes/invalidates an existing key.
// @Summary Revoke/Hapus API Key
// @Description Menghapus atau membatalkan validitas API Key tertentu berdasarkan ID.
// @Tags API Keys
// @Produce json
// @Param id path string true "API Key ID UUID"
// @Success 200 {object} map[string]interface{} "Pesan sukses penghapusan"
// @Failure 400 {object} map[string]interface{} "Format UUID tidak valid"
// @Failure 401 {object} map[string]interface{} "Unauthorized"
// @Failure 403 {object} map[string]interface{} "Akses dilarang"
// @Failure 404 {object} map[string]interface{} "API Key tidak ditemukan"
// @Security BearerAuth
// @Router /api-keys/{id} [delete]
func (h *APIKeyHandler) RevokeKey(c *fiber.Ctx) error {
	userIDStr, ok := c.Locals("user_id").(string)
	if !ok {
		return utils.Unauthorized(c, "unauthorized")
	}

	userID, err := uuid.Parse(userIDStr)
	if err != nil {
		return utils.BadRequest(c, "invalid user id")
	}

	idStr := c.Params("id")
	id, err := uuid.Parse(idStr)
	if err != nil {
		return utils.BadRequest(c, "invalid api key id")
	}

	// Verify owner first
	key, err := h.keyService.GetKey(c.Context(), id)
	if err != nil {
		return utils.NotFound(c, err.Error())
	}
	if key.UserID != userID {
		return utils.Forbidden(c, "you do not have permission to revoke this api key")
	}

	err = h.keyService.RevokeKey(c.Context(), id)
	if err != nil {
		return utils.InternalError(c, err.Error())
	}

	// Audit log
	utils.LogAction(c.Context(), h.auditRepo, c, userID, "api_key.deleted", "api_key", &id, map[string]string{"name": key.Name})

	return utils.Success(c, fiber.Map{
		"message": "api key revoked successfully",
	})
}
