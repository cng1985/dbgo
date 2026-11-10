package auth

import "time"

type AuthLoginRequest struct {
	Username string `json:"username"`
	Password string `json:"password"`
}

type AuthRegisterRequest struct {
	Username    string `json:"username"`
	Password    string `json:"password"`
	DisplayName string `json:"display_name"`
	Email       string `json:"email"`
}

type AuthOptionsResponse struct {
	AllowRegister bool `json:"allow_register"`
}

type AuthLogoutRequest struct{}

type AuthProfileRequest struct{}

type AuthProfileUpdateRequest struct {
	DisplayName string `json:"display_name"`
	Email       string `json:"email"`
}

type AuthPasswordRequest struct {
	CurrentPassword string `json:"current_password"`
	NewPassword     string `json:"new_password"`
}

type AuthEnsureRequest struct {
	Username string `json:"username"`
	Password string `json:"password"`
}

type AuthProfileResponse struct {
	UserID       int64      `json:"user_id,string"`
	Username     string     `json:"username"`
	DisplayName  string     `json:"display_name"`
	Email        string     `json:"email"`
	Role         string     `json:"role"`
	CreatedAt    time.Time  `json:"created_at"`
	ExpiresAt    *time.Time `json:"expires_at,omitempty"`
	SessionToken string     `json:"-"`
}
