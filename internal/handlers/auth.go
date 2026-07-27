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
