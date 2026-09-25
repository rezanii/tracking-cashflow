package dto

import "time"

type RegisterRequest struct {
	Name     string `json:"name" validate:"required,min=2,max=150" example:"John Doe"`
	Email    string `json:"email" validate:"required,email,max=255" example:"user@example.com"`
	Password string `json:"password" validate:"required,password,max=72" example:"Admin123!"`
}

type LoginRequest struct {
	Email    string `json:"email" validate:"required,email" example:"user@example.com"`
	Password string `json:"password" validate:"required" example:"Admin123!"`
}

type UserResponse struct {
	ID        int64     `json:"id" example:"1"`
	Name      string    `json:"name" example:"John Doe"`
	Email     string    `json:"email" example:"user@example.com"`
	IsActive  bool      `json:"is_active" example:"true"`
	CreatedAt time.Time `json:"created_at"`
}

type LoginResponse struct {
	AccessToken string       `json:"access_token"`
	TokenType   string       `json:"token_type" example:"Bearer"`
	ExpiresAt   time.Time    `json:"expires_at"`
	User        UserResponse `json:"user"`
}
