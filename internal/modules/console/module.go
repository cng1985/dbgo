package console

import (
	"go.uber.org/fx"

	"github.com/cng1985/dbgo/internal/modules/console/backup"
	"github.com/cng1985/dbgo/internal/modules/console/catalog"
	"github.com/cng1985/dbgo/internal/modules/console/connection"
	"github.com/cng1985/dbgo/internal/modules/console/migration"
	"github.com/cng1985/dbgo/internal/modules/console/overview"
	"github.com/cng1985/dbgo/internal/modules/console/query"
)

func migrationCleaner(repo *migration.Repository) connection.MigrationCleaner { return repo }

func migrationNames(resource *connection.Resource) migration.ConnectionReader { return resource }

func backupNames(resource *connection.Resource) backup.ConnectionReader { return resource }

// Module 只组合控制台子模块，不承载业务实现。
var Module = fx.Module(
	"console",
	fx.Provide(migrationCleaner, migrationNames, backupNames),
	connection.Module,
	query.Module,
	catalog.Module,
	migration.Module,
	backup.Module,
	overview.Module,
)
