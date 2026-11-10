package migration

func MigrationEntityToSimple(entity *DataMigration) *MigrationSimple {
	return &MigrationSimple{
		ID:             entity.ID,
		ConnectionID:   entity.ConnectionID,
		ConnectionName: entity.ConnectionName,
		VersionNo:      entity.VersionNo,
		Name:           entity.Name,
		Status:         entity.Status,
		Checksum:       entity.Checksum,
		AppliedAt:      entity.AppliedAt,
		CreatedAt:      entity.CreatedAt,
		UpdatedAt:      entity.UpdatedAt,
	}
}

func MigrationEntityToResponse(entity *DataMigration) *MigrationResponse {
	return &MigrationResponse{
		ID:             entity.ID,
		ConnectionID:   entity.ConnectionID,
		ConnectionName: entity.ConnectionName,
		VersionNo:      entity.VersionNo,
		Name:           entity.Name,
		UpSQL:          entity.UpSQL,
		DownSQL:        entity.DownSQL,
		Checksum:       entity.Checksum,
		Status:         entity.Status,
		ErrorMessage:   entity.ErrorMessage,
		AppliedAt:      entity.AppliedAt,
		CreatedAt:      entity.CreatedAt,
		UpdatedAt:      entity.UpdatedAt,
	}
}
