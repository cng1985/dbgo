package snippet

func toSimple(entity *Snippet) SnippetSimple {
	return SnippetSimple{
		ID:          entity.ID,
		Name:        entity.Name,
		Description: entity.Description,
		Driver:      entity.Driver,
		Version:     entity.Version,
		CreatedAt:   entity.CreatedAt,
		UpdatedAt:   entity.UpdatedAt,
	}
}

func toResponse(entity *Snippet) SnippetResponse {
	return SnippetResponse{
		ID:          entity.ID,
		Name:        entity.Name,
		Description: entity.Description,
		Body:        entity.Body,
		Driver:      entity.Driver,
		Version:     entity.Version,
		CreatedAt:   entity.CreatedAt,
		UpdatedAt:   entity.UpdatedAt,
	}
}
