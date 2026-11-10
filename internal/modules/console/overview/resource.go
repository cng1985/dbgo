package overview

import (
	"context"

	"github.com/cng1985/dbgo/internal/modules/console/backup"
	"github.com/cng1985/dbgo/internal/modules/console/connection"
	"github.com/cng1985/dbgo/internal/modules/console/migration"
	"github.com/cng1985/dbgo/internal/modules/console/query"
	"github.com/cng1985/dbgo/internal/platform/domain"
	"github.com/cng1985/dbgo/internal/platform/response"
)

type Resource struct {
	connections *connection.Resource
	queries     *query.Resource
	migrations  *migration.Resource
	backups     *backup.Resource
}

func NewResource(connections *connection.Resource, queries *query.Resource, migrations *migration.Resource, backups *backup.Resource) *Resource {
	return &Resource{connections: connections, queries: queries, migrations: migrations, backups: backups}
}

func (r *Resource) One(ctx context.Context, _ *OverviewSearch) (response.Response[OverviewResponse], error) {
	var zero response.Response[OverviewResponse]
	scope, err := domain.RequireScope(ctx)
	if err != nil {
		return zero, err
	}
	if err := scope.Allow("overview:read"); err != nil {
		return zero, err
	}
	connections, err := r.connections.Search(ctx, &connection.ConnectionSearch{Page: 1, PageSize: 1})
	if err != nil {
		return zero, err
	}
	queries, err := r.queries.Search(ctx, &query.QuerySearch{Page: 1, PageSize: 5})
	if err != nil {
		return zero, err
	}
	failed, err := r.queries.Search(ctx, &query.QuerySearch{Status: "failed", Page: 1, PageSize: 1})
	if err != nil {
		return zero, err
	}
	applied, err := r.migrations.Search(ctx, &migration.MigrationSearch{Status: "applied", Page: 1, PageSize: 1})
	if err != nil {
		return zero, err
	}
	backups, err := r.backups.Search(ctx, &backup.BackupSearch{Page: 1, PageSize: 1})
	if err != nil {
		return zero, err
	}
	recent := make([]OverviewQuery, 0, len(queries.Data))
	for _, item := range queries.Data {
		recent = append(recent, OverviewQuery{
			ID:             item.ID,
			ConnectionName: item.ConnectionName,
			Status:         item.Status,
			RowCount:       item.RowCount,
			DurationMS:     item.DurationMS,
			CreatedAt:      item.CreatedAt,
		})
	}
	return response.OK(OverviewResponse{
		ConnectionCount:       connections.Total,
		QueryCount:            queries.Total,
		FailedQueryCount:      failed.Total,
		AppliedMigrationCount: applied.Total,
		BackupCount:           backups.Total,
		RecentQueries:         recent,
	}), nil
}
