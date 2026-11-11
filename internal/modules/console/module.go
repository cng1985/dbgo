package console

import (
	"go.uber.org/fx"

	"github.com/cng1985/dbgo/internal/modules/console/backup"
	"github.com/cng1985/dbgo/internal/modules/console/catalog"
	"github.com/cng1985/dbgo/internal/modules/console/connection"
	"github.com/cng1985/dbgo/internal/modules/console/etl"
	"github.com/cng1985/dbgo/internal/modules/console/llm"
	"github.com/cng1985/dbgo/internal/modules/console/migration"
	"github.com/cng1985/dbgo/internal/modules/console/monitor"
	"github.com/cng1985/dbgo/internal/modules/console/overview"
	"github.com/cng1985/dbgo/internal/modules/console/query"
	"github.com/cng1985/dbgo/internal/modules/console/snippet"
)

func migrationCleaner(repo *migration.Repository) connection.MigrationCleaner { return repo }

func migrationNames(resource *connection.Resource) migration.ConnectionReader { return resource }

func backupNames(resource *connection.Resource) backup.ConnectionReader { return resource }

func etlNames(resource *connection.Resource) etl.ConnectionReader { return resource }

// Module 只组合控制台子模块，不承载业务实现。
var Module = fx.Module(
	"console",
	fx.Provide(migrationCleaner, migrationNames, backupNames, etlNames),
	connection.Module,
	query.Module,
	catalog.Module,
	migration.Module,
	backup.Module,
	etl.Module,
	overview.Module,
	llm.Module,
	monitor.Module,
	snippet.Module,
)
