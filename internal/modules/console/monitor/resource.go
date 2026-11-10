package monitor

import (
	"context"
	"time"

	"github.com/cng1985/dbgo/internal/modules/console/shared"
	"github.com/cng1985/dbgo/internal/platform/domain"
	apperrors "github.com/cng1985/dbgo/internal/platform/errors"
	"github.com/cng1985/dbgo/internal/platform/response"
)

type Resource struct {
	opener shared.Opener
}

func NewResource(opener shared.Opener) *Resource {
	return &Resource{opener: opener}
}

func (r *Resource) Snapshot(ctx context.Context, search *SnapshotSearch) (response.Response[SnapshotResponse], error) {
	var zero response.Response[SnapshotResponse]
	scope, err := domain.RequireScope(ctx)
	if err != nil {
		return zero, err
	}
	if err := scope.Allow("monitor:read"); err != nil {
		return zero, err
	}
	if search == nil || search.ConnectionID <= 0 {
		return zero, apperrors.Invalid("请选择连接")
	}
	target, err := r.opener.Open(ctx, search.ConnectionID)
	if err != nil {
		return zero, err
	}
	defer target.Close()
	version, err := shared.ServerVersion(ctx, target.DB, target.Driver)
	if err != nil {
		return zero, apperrors.Business(10006, apperrors.KindUnavailable, shared.PublicError("读取数据库版本失败", "", err), err)
	}
	schema := search.Schema
	if schema == "" {
		schema = target.Schema
	}
	if schema != "" {
		if _, err := shared.QuoteIdent(target.Driver, schema); err != nil {
			return zero, apperrors.Invalid("模式名无效")
		}
	}
	snap, err := shared.CollectSnapshot(ctx, target.DB, target.Driver, schema)
	if err != nil {
		return zero, apperrors.Business(10006, apperrors.KindInvalid, shared.PublicError("采集性能数据失败", "", err), err)
	}
	metrics := make([]Metric, 0, len(snap.Metrics))
	for _, item := range snap.Metrics {
		metrics = append(metrics, Metric{Key: item.Key, Label: item.Label, Value: item.Value, Unit: item.Unit, Hint: item.Hint})
	}
	sessions := make([]SessionInfo, 0, len(snap.Sessions))
	for _, item := range snap.Sessions {
		sessions = append(sessions, SessionInfo{ID: item.ID, User: item.User, Database: item.Database, State: item.State, Duration: item.Duration, SQL: item.SQL})
	}
	sizes := make([]SizeInfo, 0, len(snap.Sizes))
	for _, item := range snap.Sizes {
		sizes = append(sizes, SizeInfo{Schema: item.Schema, Name: item.Name, Rows: item.Rows, Bytes: item.Bytes})
	}
	notes := snap.Notes
	if notes == nil {
		notes = []string{}
	}
	return response.OK(SnapshotResponse{
		Driver:        target.Driver,
		ServerVersion: version,
		Schema:        schema,
		CollectedAt:   time.Now().UTC(),
		Metrics:       metrics,
		Sessions:      sessions,
		Sizes:         sizes,
		Notes:         notes,
	}), nil
}
