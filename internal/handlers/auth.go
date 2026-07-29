package handlers

import (
	"net/http"

	"github.com/google/uuid"
	"github.com/school/attendance-backend/internal/auth"
	"github.com/school/attendance-backend/internal/dto"
	"github.com/school/attendance-backend/internal/httpx"
	"github.com/school/attendance-backend/internal/services"
)

type AuthHandler struct {
	svc *services.AuthService
}

func NewAuthHandler(svc *services.AuthService) *AuthHandler {
	return &AuthHandler{svc}
}

// Login godoc
// @Summary Authenticate user and retrieve JWT tokens
// @Description Validates credentials and returns JWT access & refresh tokens.
// @Tags Authentication
// @Accept json
// @Produce json
// @Param request body dto.LoginRequest true "Login credentials"
// @Success 200 {object} httpx.Envelope{data=dto.LoginResponse}
// @Failure 400 {object} httpx.Envelope{error=httpx.ErrBody}
// @Failure 401 {object} httpx.Envelope{error=httpx.ErrBody}
// @Router /api/v1/auth/login [post]
func (h *AuthHandler) Login(w http.ResponseWriter, r *http.Request) {
	var req dto.LoginRequest
	if err := dto.Decode(r, &req); err != nil {
		httpx.WriteError(w, err)
		return
	}
	resp, err := h.svc.Login(r.Context(), &req)
	if err != nil {
		httpx.WriteError(w, err)
		return
	}
	httpx.WriteOK(w, resp)
}

// Refresh godoc
// @Summary Refresh JWT access token
// @Description Exchanges a valid refresh token for a new access/refresh token pair.
// @Tags Authentication
// @Accept json
// @Produce json
// @Param request body dto.RefreshRequest true "Refresh token"
// @Success 200 {object} httpx.Envelope{data=dto.LoginResponse}
// @Failure 400 {object} httpx.Envelope{error=httpx.ErrBody}
// @Failure 401 {object} httpx.Envelope{error=httpx.ErrBody}
// @Router /api/v1/auth/refresh [post]
func (h *AuthHandler) Refresh(w http.ResponseWriter, r *http.Request) {
	var req dto.RefreshRequest
	if err := dto.Decode(r, &req); err != nil {
		httpx.WriteError(w, err)
		return
	}
	resp, err := h.svc.Refresh(r.Context(), req.RefreshToken)
	if err != nil {
		httpx.WriteError(w, err)
		return
	}
	httpx.WriteOK(w, resp)
}

// Logout godoc
// @Summary Revoke refresh token / Logout
// @Description Revokes the specified refresh token, logging out the user session.
// @Tags Authentication
// @Accept json
// @Produce json
// @Security Bearer
// @Param request body dto.LogoutRequest true "Refresh token to revoke"
// @Success 204 "No Content"
// @Failure 400 {object} httpx.Envelope{error=httpx.ErrBody}
// @Failure 401 {object} httpx.Envelope{error=httpx.ErrBody}
// @Router /api/v1/auth/logout [post]
func (h *AuthHandler) Logout(w http.ResponseWriter, r *http.Request) {
	var req dto.LogoutRequest
	if err := dto.Decode(r, &req); err != nil {
		httpx.WriteError(w, err)
		return
	}
	if err := h.svc.Logout(r.Context(), req.RefreshToken); err != nil {
		httpx.WriteError(w, err)
		return
	}
	httpx.WriteNoContent(w)
}

// Me godoc
// @Summary Get current user profile
// @Description Retrieves the authenticated user's profile information.
// @Tags Authentication
// @Accept json
// @Produce json
// @Security Bearer
// @Success 200 {object} httpx.Envelope{data=dto.UserResp}
// @Failure 401 {object} httpx.Envelope{error=httpx.ErrBody}
// @Router /api/v1/me [get]
func (h *AuthHandler) Me(w http.ResponseWriter, r *http.Request) {
	claims := auth.ClaimsFromContext(r.Context())
	userID, _ := uuid.Parse(claims.Subject)
	user, err := h.svc.GetMe(r.Context(), userID)
	if err != nil {
		httpx.WriteError(w, err)
		return
	}
	httpx.WriteOK(w, user)
}

// UpdateProfile godoc
// @Summary Update current user profile
// @Description Updates the authenticated user's full name and/or phone number.
// @Tags Authentication
// @Accept json
// @Produce json
// @Security Bearer
// @Param request body dto.UpdateProfileRequest true "Profile details"
// @Success 200 {object} httpx.Envelope{data=dto.UserResp}
// @Failure 400 {object} httpx.Envelope{error=httpx.ErrBody}
// @Failure 401 {object} httpx.Envelope{error=httpx.ErrBody}
// @Router /api/v1/users/me [put]
func (h *AuthHandler) UpdateProfile(w http.ResponseWriter, r *http.Request) {
	claims := auth.ClaimsFromContext(r.Context())
	userID, _ := uuid.Parse(claims.Subject)

	var req dto.UpdateProfileRequest
	if err := dto.Decode(r, &req); err != nil {
		httpx.WriteError(w, err)
		return
	}

	user, err := h.svc.UpdateProfile(r.Context(), userID, &req)
	if err != nil {
		httpx.WriteError(w, err)
		return
	}
	httpx.WriteOK(w, user)
}

// ChangePassword godoc
// @Summary Change user password
// @Description Changes the authenticated user's password.
// @Tags Authentication
// @Accept json
// @Produce json
// @Security Bearer
// @Param request body dto.ChangePasswordRequest true "Password details"
// @Success 204 "No Content"
// @Failure 400 {object} httpx.Envelope{error=httpx.ErrBody}
// @Failure 401 {object} httpx.Envelope{error=httpx.ErrBody}
// @Router /api/v1/users/me/change-password [post]
func (h *AuthHandler) ChangePassword(w http.ResponseWriter, r *http.Request) {
	claims := auth.ClaimsFromContext(r.Context())
	userID, _ := uuid.Parse(claims.Subject)

	var req dto.ChangePasswordRequest
	if err := dto.Decode(r, &req); err != nil {
		httpx.WriteError(w, err)
		return
	}

	if err := h.svc.ChangePassword(r.Context(), userID, &req); err != nil {
		httpx.WriteError(w, err)
		return
	}
	httpx.WriteNoContent(w)
}
