package connection

import (
	"context"

	"github.com/cng1985/dbgo/internal/modules/console/shared"
	"github.com/cng1985/dbgo/internal/platform/domain"
	apperrors "github.com/cng1985/dbgo/internal/platform/errors"
	"github.com/cng1985/dbgo/internal/platform/secret"
)

type TargetOpener struct {
	repo *Repository
	box  *secret.Box
}

func NewTargetOpener(repo *Repository, box *secret.Box) *TargetOpener {
	return &TargetOpener{repo: repo, box: box}
}

func (o *TargetOpener) Open(ctx context.Context, connectionID int64) (*shared.Target, error) {
	scope, err := domain.RequireScope(ctx)
	if err != nil {
		return nil, err
	}
	if err := scope.Allow("connection:use"); err != nil {
		return nil, err
	}
	if connectionID <= 0 {
		return nil, apperrors.Invalid("连接主键无效")
	}
	entity, err := o.repo.find(ctx, scope.ActorID, connectionID)
	if err != nil {
		return nil, err
	}
	password, err := o.box.Decrypt(entity.PasswordCipher)
	if err != nil {
		return nil, apperrors.Unavailable("连接口令无法解密", err)
	}
	db, err := shared.Open(ctx, shared.Endpoint{
		Driver:     entity.Driver,
		Host:       entity.Host,
		Port:       entity.Port,
		Database:   entity.DatabaseName,
		Username:   entity.Username,
		Password:   password,
		SQLitePath: entity.SQLitePath,
		SSLMode:    entity.SSLMode,
	})
	if err != nil {
		return nil, apperrors.Business(10001, apperrors.KindUnavailable, shared.PublicError("连接目标数据库失败", password, err), err)
	}
	return &shared.Target{
		Driver: entity.Driver,
		Name:   entity.Name,
		Schema: defaultSchema(entity),
		DB:     db,
	}, nil
}

func defaultSchema(entity *Connection) string {
	switch entity.Driver {
	case "sqlite":
		return "main"
	case "postgres":
		return "public"
	case "sqlserver":
		return "dbo"
	default:
		return entity.DatabaseName
	}
}
