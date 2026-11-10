package llm

func toSimple(entity *Profile) ProfileSimple {
	return ProfileSimple{
		ID:          entity.ID,
		Name:        entity.Name,
		BaseURL:     entity.BaseURL,
		Model:       entity.Model,
		Temperature: entity.Temperature,
		MaxTokens:   entity.MaxTokens,
		IsDefault:   entity.IsDefault,
		APIKeySet:   entity.APIKeyCipher != "",
		Version:     entity.Version,
		CreatedAt:   entity.CreatedAt,
		UpdatedAt:   entity.UpdatedAt,
	}
}

func toResponse(entity *Profile) ProfileResponse {
	simple := toSimple(entity)
	return ProfileResponse{
		ID:          simple.ID,
		Name:        simple.Name,
		BaseURL:     simple.BaseURL,
		Model:       simple.Model,
		Temperature: simple.Temperature,
		MaxTokens:   simple.MaxTokens,
		IsDefault:   simple.IsDefault,
		Remark:      entity.Remark,
		APIKeySet:   simple.APIKeySet,
		Version:     simple.Version,
		CreatedAt:   simple.CreatedAt,
		UpdatedAt:   simple.UpdatedAt,
	}
}
