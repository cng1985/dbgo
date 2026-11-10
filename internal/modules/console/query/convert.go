package query

func QueryEntityToSimple(entity *QueryHistory) *QuerySimple {
	return &QuerySimple{
		ID:             entity.ID,
		ConnectionID:   entity.ConnectionID,
		ConnectionName: entity.ConnectionName,
		Status:         entity.Status,
		RowCount:       entity.RowCount,
		DurationMS:     entity.DurationMS,
		ReadOnly:       entity.ReadOnly,
		ErrorMessage:   entity.ErrorMessage,
		CreatedAt:      entity.CreatedAt,
	}
}

func QueryEntityToResponse(entity *QueryHistory) *QueryResponse {
	return &QueryResponse{
		ID:             entity.ID,
		ConnectionID:   entity.ConnectionID,
		ConnectionName: entity.ConnectionName,
		SQL:            entity.SQLText,
		Status:         entity.Status,
		ReadOnly:       entity.ReadOnly,
		DurationMS:     entity.DurationMS,
		RowCount:       entity.RowCount,
		ErrorMessage:   entity.ErrorMessage,
		Statements:     []QueryStatement{},
		CreatedAt:      entity.CreatedAt,
	}
}
