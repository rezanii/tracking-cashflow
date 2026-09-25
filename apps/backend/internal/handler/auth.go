package handler

import (
	"net/http"

	"github.com/rezanii/tracking-cashflow/apps/backend/internal/dto"
	"github.com/rezanii/tracking-cashflow/apps/backend/internal/middleware"
	"github.com/rezanii/tracking-cashflow/apps/backend/internal/service"
	"github.com/rezanii/tracking-cashflow/apps/backend/internal/utils"
	"github.com/rezanii/tracking-cashflow/apps/backend/internal/validator"
)

type AuthHandler struct {
	auth      service.AuthService
	validator *validator.Validator
}

func NewAuthHandler(auth service.AuthService, v *validator.Validator) *AuthHandler {
	return &AuthHandler{auth: auth, validator: v}
}

// Register creates an account.
//
//	@Summary		Register a new user
//	@Tags			Auth
//	@Accept			json
//	@Produce		json
//	@Param			request	body		dto.RegisterRequest	true	"Registration payload"
//	@Success		201		{object}	utils.Envelope{data=dto.UserResponse}
//	@Failure		409		{object}	utils.Envelope
//	@Failure		422		{object}	utils.Envelope
//	@Router			/auth/register [post]
func (h *AuthHandler) Register(w http.ResponseWriter, r *http.Request) {
	var request dto.RegisterRequest
	if err := decode(r, h.validator, &request); err != nil {
		utils.WriteError(w, err)
		return
	}

	user, err := h.auth.Register(r.Context(), request)
	if err != nil {
		utils.WriteError(w, err)
		return
	}
	utils.Created(w, "Registration successful", user)
}

// Login exchanges credentials for an access token.
//
//	@Summary		Log in
//	@Tags			Auth
//	@Accept			json
//	@Produce		json
//	@Param			request	body		dto.LoginRequest	true	"Login payload"
//	@Success		200		{object}	utils.Envelope{data=dto.LoginResponse}
//	@Failure		401		{object}	utils.Envelope
//	@Failure		422		{object}	utils.Envelope
//	@Router			/auth/login [post]
func (h *AuthHandler) Login(w http.ResponseWriter, r *http.Request) {
	var request dto.LoginRequest
	if err := decode(r, h.validator, &request); err != nil {
		utils.WriteError(w, err)
		return
	}

	result, err := h.auth.Login(r.Context(), request)
	if err != nil {
		utils.WriteError(w, err)
		return
	}
	utils.OK(w, "Login successful", result)
}

// Logout is a client-side token drop; the endpoint exists so the frontend has a single
// place to call and so an audit log can record the intent.
//
//	@Summary		Log out
//	@Tags			Auth
//	@Produce		json
//	@Security		BearerAuth
//	@Success		200	{object}	utils.Envelope
//	@Failure		401	{object}	utils.Envelope
//	@Router			/auth/logout [post]
func (h *AuthHandler) Logout(w http.ResponseWriter, r *http.Request) {
	if _, err := middleware.UserID(r.Context()); err != nil {
		utils.WriteError(w, err)
		return
	}
	utils.OK(w, "Logout successful", nil)
}

// Me returns the authenticated user.
//
//	@Summary		Current user
//	@Tags			Auth
//	@Produce		json
//	@Security		BearerAuth
//	@Success		200	{object}	utils.Envelope{data=dto.UserResponse}
//	@Failure		401	{object}	utils.Envelope
//	@Router			/auth/me [get]
func (h *AuthHandler) Me(w http.ResponseWriter, r *http.Request) {
	userID, err := middleware.UserID(r.Context())
	if err != nil {
		utils.WriteError(w, err)
		return
	}

	user, err := h.auth.Me(r.Context(), userID)
	if err != nil {
		utils.WriteError(w, err)
		return
	}
	utils.OK(w, "Data retrieved successfully", user)
}
