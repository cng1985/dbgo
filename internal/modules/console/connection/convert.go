package connection

func ConnectionEntityToSimple(entity *Connection) *ConnectionSimple {
	return &ConnectionSimple{
		ID:           entity.ID,
		Name:         entity.Name,
		Driver:       entity.Driver,
		Host:         entity.Host,
		Port:         entity.Port,
		DatabaseName: entity.DatabaseName,
		Username:     entity.Username,
		SQLitePath:   entity.SQLitePath,
		SSLMode:      entity.SSLMode,
		PasswordSet:  entity.PasswordCipher != "",
		Version:      entity.Version,
		CreatedAt:    entity.CreatedAt,
		UpdatedAt:    entity.UpdatedAt,
	}
}

func ConnectionEntityToResponse(entity *Connection) *ConnectionResponse {
	simple := ConnectionEntityToSimple(entity)
	return &ConnectionResponse{
		ID:           simple.ID,
		Name:         simple.Name,
		Driver:       simple.Driver,
		Host:         simple.Host,
		Port:         simple.Port,
		DatabaseName: simple.DatabaseName,
		Username:     simple.Username,
		SQLitePath:   simple.SQLitePath,
		SSLMode:      simple.SSLMode,
		Remark:       entity.Remark,
		PasswordSet:  simple.PasswordSet,
		Version:      simple.Version,
		CreatedAt:    simple.CreatedAt,
		UpdatedAt:    simple.UpdatedAt,
	}
}
