package auth

import "time"

func UserEntityToProfile(entity *User, expiresAt *time.Time) *AuthProfileResponse {
	return &AuthProfileResponse{
		UserID:      entity.ID,
		Username:    entity.Username,
		DisplayName: entity.DisplayName,
		Email:       entity.Email,
		Role:        entity.RoleCode,
		CreatedAt:   entity.CreatedAt,
		ExpiresAt:   expiresAt,
	}
}
