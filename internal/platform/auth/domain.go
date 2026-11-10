package auth

import "time"

type AuthLoginRequest struct {
	Username string `json:"username"`
	Password string `json:"password"`
}

type AuthLogoutRequest struct{}

type AuthProfileRequest struct{}

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
	Role         string     `json:"role"`
	ExpiresAt    *time.Time `json:"expires_at,omitempty"`
	SessionToken string     `json:"-"`
}
