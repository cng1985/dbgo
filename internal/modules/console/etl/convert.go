package etl

import "encoding/json"

func toSimple(entity *Task, run *Run) TaskSimple {
	item := TaskSimple{
		ID:                   entity.ID,
		Name:                 entity.Name,
		SourceConnectionID:   entity.SourceConnectionID,
		SourceConnectionName: entity.SourceConnectionName,
		TargetConnectionID:   entity.TargetConnectionID,
		TargetConnectionName: entity.TargetConnectionName,
		TargetSchema:         entity.TargetSchema,
		TargetTable:          entity.TargetTable,
		WriteMode:            entity.WriteMode,
		Version:              entity.Version,
		UpdatedAt:            entity.UpdatedAt,
	}
	if run != nil {
		item.LastStatus = run.Status
		item.LastRowsRead = run.RowsRead
		item.LastRowsWritten = run.RowsWritten
		item.LastError = run.ErrorMessage
		item.LastFinishedAt = run.FinishedAt
	}
	return item
}

func toResponse(entity *Task, mappings []ColumnMap) TaskResponse {
	if mappings == nil {
		mappings = []ColumnMap{}
	}
	return TaskResponse{
		ID:                   entity.ID,
		Name:                 entity.Name,
		SourceConnectionID:   entity.SourceConnectionID,
		SourceConnectionName: entity.SourceConnectionName,
		SourceSQL:            entity.SourceSQL,
		TargetConnectionID:   entity.TargetConnectionID,
		TargetConnectionName: entity.TargetConnectionName,
		TargetSchema:         entity.TargetSchema,
		TargetTable:          entity.TargetTable,
		WriteMode:            entity.WriteMode,
		Mappings:             mappings,
		Version:              entity.Version,
		CreatedAt:            entity.CreatedAt,
		UpdatedAt:            entity.UpdatedAt,
	}
}

func toRun(entity *Run) RunResponse {
	return RunResponse{
		ID:           entity.ID,
		TaskID:       entity.TaskID,
		Status:       entity.Status,
		RowsRead:     entity.RowsRead,
		RowsWritten:  entity.RowsWritten,
		ErrorMessage: entity.ErrorMessage,
		StartedAt:    entity.StartedAt,
		FinishedAt:   entity.FinishedAt,
	}
}

func decodeMappings(raw string) ([]ColumnMap, error) {
	if raw == "" {
		return []ColumnMap{}, nil
	}
	var items []ColumnMap
	if err := json.Unmarshal([]byte(raw), &items); err != nil {
		return nil, err
	}
	if items == nil {
		items = []ColumnMap{}
	}
	return items, nil
}
