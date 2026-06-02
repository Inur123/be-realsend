package handler

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"

	"github.com/gofiber/fiber/v2"
	"github.com/google/uuid"
	"github.com/realsend/be-realsend/internal/config"
	"github.com/realsend/be-realsend/internal/repository"
	"github.com/realsend/be-realsend/internal/service"
	"github.com/realsend/be-realsend/internal/utils"
)

type AuthHandler struct {
	cfg         *config.Config
	authService service.AuthService
	auditRepo   repository.AuditLogRepository
}

func NewAuthHandler(cfg *config.Config, authService service.AuthService, auditRepo repository.AuditLogRepository) *AuthHandler {
	return &AuthHandler{
		cfg:         cfg,
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
// @Summary Registrasi akun baru
// @Description Mendaftarkan akun developer baru di platform RealSend.
// @Tags Auth
// @Accept json
// @Produce json
// @Param request body registerRequest true "Data registrasi user"
// @Success 201 {object} map[string]interface{} "Akun berhasil dibuat"
// @Failure 400 {object} map[string]interface{} "Body request tidak valid"
// @Failure 409 {object} map[string]interface{} "Email sudah terdaftar"
// @Failure 422 {object} map[string]interface{} "Validasi gagal"
// @Router /auth/register [post]
func (h *AuthHandler) Register(c *fiber.Ctx) error {
	var req registerRequest
	if err := c.BodyParser(&req); err != nil {
		return utils.BadRequest(c, "format data tidak valid")
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
// @Summary Login akun
// @Description Masuk ke akun menggunakan email dan password untuk mendapatkan JWT token.
// @Tags Auth
// @Accept json
// @Produce json
// @Param request body loginRequest true "Data login"
// @Success 200 {object} map[string]interface{} "Token JWT dan data profil user"
// @Failure 400 {object} map[string]interface{} "Body request tidak valid"
// @Failure 401 {object} map[string]interface{} "Email atau password salah"
// @Failure 422 {object} map[string]interface{} "Validasi gagal"
// @Router /auth/login [post]
func (h *AuthHandler) Login(c *fiber.Ctx) error {
	var req loginRequest
	if err := c.BodyParser(&req); err != nil {
		return utils.BadRequest(c, "format data tidak valid")
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
// @Summary Dapatkan profil user saat ini
// @Description Mendapatkan detail informasi profil user yang sedang login berdasarkan JWT token.
// @Tags Auth
// @Produce json
// @Success 200 {object} map[string]interface{} "Detail profil user"
// @Failure 401 {object} map[string]interface{} "Unauthorized"
// @Security BearerAuth
// @Router /auth/me [get]
func (h *AuthHandler) Me(c *fiber.Ctx) error {
	userIDStr, ok := c.Locals("user_id").(string)
	if !ok {
		return utils.Unauthorized(c, "sesi tidak valid atau kedaluwarsa")
	}

	userID, err := uuid.Parse(userIDStr)
	if err != nil {
		return utils.BadRequest(c, "ID pengguna tidak valid")
	}

	user, err := h.authService.GetProfile(c.Context(), userID)
	if err != nil {
		return utils.NotFound(c, err.Error())
	}

	return utils.Success(c, user)
}

// UpdateProfile handles profile modifications.
// @Summary Perbarui profil user
// @Description Mengubah nama lengkap, nama perusahaan, atau email user.
// @Tags Auth
// @Accept json
// @Produce json
// @Param request body updateProfileRequest true "Data profil baru"
// @Success 200 {object} map[string]interface{} "Profil berhasil diperbarui"
// @Failure 401 {object} map[string]interface{} "Unauthorized"
// @Failure 422 {object} map[string]interface{} "Validasi gagal"
// @Security BearerAuth
// @Router /auth/me [put]
func (h *AuthHandler) UpdateProfile(c *fiber.Ctx) error {
	userIDStr, ok := c.Locals("user_id").(string)
	if !ok {
		return utils.Unauthorized(c, "sesi tidak valid atau kedaluwarsa")
	}

	userID, err := uuid.Parse(userIDStr)
	if err != nil {
		return utils.BadRequest(c, "ID pengguna tidak valid")
	}

	var req updateProfileRequest
	if err := c.BodyParser(&req); err != nil {
		return utils.BadRequest(c, "format data tidak valid")
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
// @Summary Ubah password user
// @Description Mengubah password lama dengan password baru.
// @Tags Auth
// @Accept json
// @Produce json
// @Param request body changePasswordRequest true "Data password"
// @Success 200 {object} map[string]interface{} "Password berhasil diperbarui"
// @Failure 400 {object} map[string]interface{} "Password lama salah atau validasi gagal"
// @Failure 401 {object} map[string]interface{} "Unauthorized"
// @Security BearerAuth
// @Router /auth/me/password [put]
func (h *AuthHandler) ChangePassword(c *fiber.Ctx) error {
	userIDStr, ok := c.Locals("user_id").(string)
	if !ok {
		return utils.Unauthorized(c, "sesi tidak valid atau kedaluwarsa")
	}

	userID, err := uuid.Parse(userIDStr)
	if err != nil {
		return utils.BadRequest(c, "ID pengguna tidak valid")
	}

	var req changePasswordRequest
	if err := c.BodyParser(&req); err != nil {
		return utils.BadRequest(c, "format data tidak valid")
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
// @Summary Logout user
// @Description Keluar dari sistem dan membuat log aktivitas logout.
// @Tags Auth
// @Produce json
// @Success 200 {object} map[string]interface{} "Logout berhasil"
// @Failure 401 {object} map[string]interface{} "Unauthorized"
// @Security BearerAuth
// @Router /auth/logout [post]
func (h *AuthHandler) Logout(c *fiber.Ctx) error {
	userIDStr, ok := c.Locals("user_id").(string)
	if !ok {
		return utils.Unauthorized(c, "sesi tidak valid atau kedaluwarsa")
	}

	userID, err := uuid.Parse(userIDStr)
	if err != nil {
		return utils.BadRequest(c, "ID pengguna tidak valid")
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

// GoogleLogin redirects to Google consent page or serves a beautiful mock login screen in development.
func (h *AuthHandler) GoogleLogin(c *fiber.Ctx) error {
	if h.cfg.GoogleClientID == "" {
		// Mock Google Sign-In Page for development ease
		c.Set("Content-Type", "text/html; charset=utf-8")
		return c.SendString(`<!DOCTYPE html>
<html>
<head>
  <title>RealSend - Google Auth Simulator</title>
  <meta name="viewport" content="width=device-width, initial-scale=1.0">
  <link href="https://fonts.googleapis.com/css2?family=Plus+Jakarta+Sans:wght@300;400;500;600;700&display=swap" rel="stylesheet">
  <style>
    :root {
      --bg: #090d16;
      --card-bg: #101622;
      --border: rgba(255, 255, 255, 0.08);
      --text: #f3f4f6;
      --text-muted: #9ca3af;
      --google-blue: #4285F4;
    }
    body {
      font-family: 'Plus Jakarta Sans', sans-serif;
      background: var(--bg);
      color: var(--text);
      display: flex;
      align-items: center;
      justify-content: center;
      min-height: 100vh;
      margin: 0;
      padding: 1rem;
      box-sizing: border-box;
    }
    .card {
      background: var(--card-bg);
      border: 1px solid var(--border);
      padding: 2.5rem 2rem;
      border-radius: 16px;
      width: 100%;
      max-width: 440px;
      text-align: center;
      box-shadow: 0 10px 25px -5px rgba(0, 0, 0, 0.3), 0 8px 10px -6px rgba(0, 0, 0, 0.3);
    }
    .logo {
      font-weight: 700;
      font-size: 1.5rem;
      background: linear-gradient(135deg, #3b82f6 0%, #1d4ed8 100%);
      -webkit-background-clip: text;
      -webkit-text-fill-color: transparent;
      margin-bottom: 0.5rem;
    }
    h1 {
      font-size: 1.25rem;
      margin: 0 0 1.5rem 0;
      font-weight: 600;
    }
    .desc {
      color: var(--text-muted);
      font-size: 0.875rem;
      line-height: 1.5;
      margin-bottom: 2rem;
    }
    .form-group {
      text-align: left;
      margin-bottom: 1.25rem;
    }
    label {
      display: block;
      font-size: 0.875rem;
      margin-bottom: 0.5rem;
      color: #d1d5db;
    }
    input {
      width: 100%;
      padding: 0.75rem 1rem;
      border-radius: 8px;
      border: 1px solid var(--border);
      background: #172030;
      color: #fff;
      font-size: 0.95rem;
      box-sizing: border-box;
      transition: border-color 0.2s;
    }
    input:focus {
      border-color: #3b82f6;
      outline: none;
    }
    .btn {
      width: 100%;
      padding: 0.85rem;
      background: var(--google-blue);
      color: white;
      border: none;
      border-radius: 8px;
      font-size: 0.95rem;
      font-weight: 600;
      cursor: pointer;
      display: flex;
      align-items: center;
      justify-content: center;
      gap: 0.75rem;
      margin-top: 1.5rem;
      box-shadow: 0 4px 12px rgba(66, 133, 244, 0.25);
      transition: filter 0.2s;
    }
    .btn:hover {
      filter: brightness(1.1);
    }
  </style>
</head>
<body>
  <div class="card">
    <div class="logo">RealSend</div>
    <h1>Google Account Simulator</h1>
    <p class="desc">Anda masuk ke Simulator Google Auth karena <code>GOOGLE_CLIENT_ID</code> belum dikonfigurasi di file <code>.env</code> Anda. Sempurna untuk pengujian lokal!</p>
    <form action="/api/v1/auth/google/callback" method="GET">
      <div class="form-group">
        <label>Nama Lengkap</label>
        <input type="text" name="name" value="Developer RealSend" required />
      </div>
      <div class="form-group">
        <label>Alamat Email Google</label>
        <input type="email" name="email" value="developer@realsend.web.id" required />
      </div>
      <input type="hidden" name="code" value="mock_google_flow_success" />
      <button type="submit" class="btn">
        <svg width="18" height="18" viewBox="0 0 24 24" fill="none" xmlns="http://www.w3.org/2000/svg">
          <path d="M22.56 12.25c0-.78-.07-1.53-.2-2.25H12v4.26h5.92c-.26 1.37-1.04 2.53-2.21 3.31v2.77h3.57c2.08-1.92 3.28-4.74 3.28-8.09z" fill="#4285F4"/>
          <path d="M12 23c2.97 0 5.46-.98 7.28-2.66l-3.57-2.77c-.98.66-2.23 1.06-3.71 1.06-2.86 0-5.29-1.93-6.16-4.53H2.18v2.84C3.99 20.53 7.7 23 12 23z" fill="#34A853"/>
          <path d="M5.84 14.09c-.22-.66-.35-1.36-.35-2.09s.13-1.43.35-2.09V7.06H2.18C1.43 8.55 1 10.22 1 12s.43 3.45 1.18 4.94l2.85-2.22c-.87-2.6-2.87-4.53-5.85-4.53z" fill="#FBBC05"/>
          <path d="M12 5.38c1.62 0 3.06.56 4.21 1.64l3.15-3.15C17.45 2.09 14.97 1 12 1 7.7 1 3.99 3.47 2.18 7.06l3.66 2.84c.87-2.6 3.3-4.52 6.16-4.52z" fill="#EA4335"/>
        </svg>
        Lanjutkan dengan Google Simulator
      </button>
    </form>
  </div>
</body>
</html>`)
	}

	// Real Google Auth URL creation
	googleAuthURL := "https://accounts.google.com/o/oauth2/v2/auth"
	u, _ := url.Parse(googleAuthURL)
	q := u.Query()
	q.Set("client_id", h.cfg.GoogleClientID)
	q.Set("redirect_uri", h.cfg.GoogleRedirectURL)
	q.Set("response_type", "code")
	q.Set("scope", "openid email profile")
	q.Set("state", uuid.New().String())
	u.RawQuery = q.Encode()

	return c.Redirect(u.String(), http.StatusTemporaryRedirect)
}

// GoogleCallback handles redirect from Google auth or Google simulator.
func (h *AuthHandler) GoogleCallback(c *fiber.Ctx) error {
	var email string
	var name string

	redirectToLoginError := func(errMsg string) error {
		frontendLoginURL := fmt.Sprintf("%s/login", h.cfg.CORSOrigins)
		redirectURL := fmt.Sprintf("%s?error=%s", frontendLoginURL, url.QueryEscape(errMsg))
		return c.Redirect(redirectURL, http.StatusTemporaryRedirect)
	}

	code := c.Query("code")
	if code == "" {
		return redirectToLoginError("missing authorization code")
	}

	if code == "mock_google_flow_success" {
		email = c.Query("email")
		name = c.Query("name")
		if email == "" {
			email = "developer@realsend.web.id"
		}
		if name == "" {
			name = "Developer RealSend"
		}
	} else {
		// Real Google Exchange
		// 1. Post to exchange token
		tokenURL := "https://oauth2.googleapis.com/token"
		resp, err := http.PostForm(tokenURL, url.Values{
			"client_id":     {h.cfg.GoogleClientID},
			"client_secret": {h.cfg.GoogleClientSecret},
			"code":          {code},
			"grant_type":    {"authorization_code"},
			"redirect_uri":  {h.cfg.GoogleRedirectURL},
		})
		if err != nil {
			return redirectToLoginError(fmt.Sprintf("failed to request token: %v", err))
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusOK {
			body, _ := io.ReadAll(resp.Body)
			return redirectToLoginError(fmt.Sprintf("failed to exchange token: %s", string(body)))
		}

		var tokenResp struct {
			AccessToken string `json:"access_token"`
		}
		if err := json.NewDecoder(resp.Body).Decode(&tokenResp); err != nil {
			return redirectToLoginError("failed to decode token response")
		}

		// 2. Fetch user profile
		profileURL := "https://www.googleapis.com/oauth2/v2/userinfo"
		req, err := http.NewRequest("GET", profileURL, nil)
		if err != nil {
			return redirectToLoginError("failed to create profile request")
		}
		req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", tokenResp.AccessToken))

		profileResp, err := http.DefaultClient.Do(req)
		if err != nil || profileResp.StatusCode != http.StatusOK {
			return redirectToLoginError("failed to get profile from Google")
		}
		defer profileResp.Body.Close()

		var profile struct {
			Email string `json:"email"`
			Name  string `json:"name"`
		}
		if err := json.NewDecoder(profileResp.Body).Decode(&profile); err != nil {
			return redirectToLoginError("failed to decode user profile")
		}

		email = profile.Email
		name = profile.Name
	}

	if email == "" {
		return redirectToLoginError("email not retrieved from google account")
	}

	// Sign in or register via our AuthService
	token, user, err := h.authService.LoginOrRegisterGoogle(c.Context(), email, name)
	if err != nil {
		return redirectToLoginError(err.Error())
	}

	// Audit log
	utils.LogAction(c.Context(), h.auditRepo, c, user.ID, "auth.google", "user", &user.ID, map[string]string{"email": user.Email})

	// Redirect back to frontend callback URL
	frontendURL := fmt.Sprintf("%s/auth/callback", h.cfg.CORSOrigins)
	redirectURL := fmt.Sprintf("%s?token=%s", frontendURL, url.QueryEscape(token))

	return c.Redirect(redirectURL, http.StatusTemporaryRedirect)
}

