package backup

func BackupEntityToSimple(entity *DataBackup) *BackupSimple {
	return &BackupSimple{
		ID:             entity.ID,
		ConnectionID:   entity.ConnectionID,
		ConnectionName: entity.ConnectionName,
		Driver:         entity.Driver,
		Name:           entity.Name,
		Format:         entity.Format,
		SizeBytes:      entity.SizeBytes,
		Checksum:       entity.Checksum,
		Status:         entity.Status,
		Truncated:      entity.Truncated,
		CreatedAt:      entity.CreatedAt,
	}
}

func BackupEntityToResponse(entity *DataBackup) *BackupResponse {
	return &BackupResponse{
		ID:             entity.ID,
		ConnectionID:   entity.ConnectionID,
		ConnectionName: entity.ConnectionName,
		Driver:         entity.Driver,
		Name:           entity.Name,
		Format:         entity.Format,
		SizeBytes:      entity.SizeBytes,
		Checksum:       entity.Checksum,
		Status:         entity.Status,
		Truncated:      entity.Truncated,
		StatementCount: entity.StatementCount,
		ErrorMessage:   entity.ErrorMessage,
		FileName:       fileName(entity),
		CreatedAt:      entity.CreatedAt,
	}
}

func fileName(entity *DataBackup) string {
	extension := "sql"
	if entity.Format == "sqlite" {
		extension = "db"
	}
	return entity.Name + "." + extension
}
