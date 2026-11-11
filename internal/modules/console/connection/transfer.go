package connection

import (
	"context"
	"fmt"
	"strconv"
	"strings"
	"time"

	apperrors "github.com/cng1985/dbgo/internal/platform/errors"
	"github.com/cng1985/dbgo/internal/platform/response"
)

const connectionDocumentFormat = "dbgo-connections"

func (r *Resource) Export(ctx context.Context, request *ConnectionExportRequest) (response.Response[ConnectionDocument], error) {
	var zero response.Response[ConnectionDocument]
	scope, err := r.authorize(ctx, "connection:read")
	if err != nil {
		return zero, err
	}
	if request == nil {
		request = &ConnectionExportRequest{}
	}
	ids, err := parseIDs(request.IDs)
	if err != nil {
		return zero, err
	}
	if len(ids) == 0 {
		total, err := r.repo.Count(ctx, scope.ActorID)
		if err != nil {
			return zero, err
		}
		if total > 500 {
			return zero, apperrors.Invalid("连接超过 500 个，请按主键分批导出")
		}
	}
	entities, err := r.repo.List(ctx, scope.ActorID, ids)
	if err != nil {
		return zero, err
	}
	if len(ids) > 0 && len(entities) != len(uniqueIDs(ids)) {
		return zero, apperrors.NotFound("有连接不存在")
	}
	items := make([]ConnectionPortable, 0, len(entities))
	for i := range entities {
		item := ConnectionPortable{
			Name:         entities[i].Name,
			Driver:       entities[i].Driver,
			Host:         entities[i].Host,
			Port:         entities[i].Port,
			DatabaseName: entities[i].DatabaseName,
			Username:     entities[i].Username,
			SQLitePath:   entities[i].SQLitePath,
			SSLMode:      entities[i].SSLMode,
			Remark:       entities[i].Remark,
		}
		if request.IncludePassword && entities[i].PasswordCipher != "" {
			password, err := r.box.Decrypt(entities[i].PasswordCipher)
			if err != nil {
				return zero, apperrors.Unavailable("连接口令无法解密", err)
			}
			item.Password = password
		}
		items = append(items, item)
	}
	return response.OK(ConnectionDocument{
		Format:      connectionDocumentFormat,
		Version:     1,
		ExportedAt:  time.Now().UTC(),
		Connections: items,
	}), nil
}

func (r *Resource) Import(ctx context.Context, request *ConnectionImportRequest) (response.Response[ConnectionImportResponse], error) {
	var zero response.Response[ConnectionImportResponse]
	scope, err := r.authorize(ctx, "connection:write")
	if err != nil {
		return zero, err
	}
	if request == nil || request.Document == nil {
		return zero, apperrors.Invalid("请提供连接文件")
	}
	if request.Document.Format != connectionDocumentFormat || request.Document.Version != 1 {
		return zero, apperrors.Invalid("连接文件格式不受支持")
	}
	if len(request.Document.Connections) == 0 {
		return zero, apperrors.Invalid("连接文件里没有连接")
	}
	if len(request.Document.Connections) > 200 {
		return zero, apperrors.Invalid("一次最多导入 200 个连接")
	}
	mode := strings.ToLower(strings.TrimSpace(request.OnConflict))
	if mode == "" {
		mode = "rename"
	}
	switch mode {
	case "skip", "rename", "error":
	default:
		return zero, apperrors.Invalid("重名策略只支持 skip、rename、error")
	}
	existing, err := r.repo.List(ctx, scope.ActorID, nil)
	if err != nil {
		return zero, err
	}
	if int64(len(existing)) >= 500 {
		return zero, apperrors.Invalid("已有连接达到 500 个，请先清理后再导入")
	}
	used := map[string]bool{}
	for i := range existing {
		used[existing[i].Name] = true
	}
	now := time.Now().UTC()
	entities := make([]*Connection, 0, len(request.Document.Connections))
	skipped := 0
	for _, item := range request.Document.Connections {
		name := strings.TrimSpace(item.Name)
		if used[name] {
			switch mode {
			case "skip":
				skipped++
				continue
			case "error":
				return zero, apperrors.Conflict("连接名称已存在：" + name)
			default:
				name = uniqueName(name, used)
			}
		}
		if name == "" {
			return zero, apperrors.Invalid("连接名称无效")
		}
		used[name] = true
		normalized, err := normalizeConnection(r.cfg, &ConnectionRequest{
			Name:         name,
			Driver:       item.Driver,
			Host:         item.Host,
			Port:         item.Port,
			DatabaseName: item.DatabaseName,
			Username:     item.Username,
			Password:     item.Password,
			SQLitePath:   item.SQLitePath,
			SSLMode:      item.SSLMode,
			Remark:       item.Remark,
		}, false)
		if err != nil {
			return zero, err
		}
		cipher, err := r.box.Encrypt(normalized.Password)
		if err != nil {
			return zero, apperrors.Internal(err)
		}
		entities = append(entities, &Connection{
			Name:           normalized.Name,
			Driver:         normalized.Driver,
			Host:           normalized.Host,
			Port:           normalized.Port,
			DatabaseName:   normalized.DatabaseName,
			Username:       normalized.Username,
			PasswordCipher: cipher,
			SQLitePath:     normalized.SQLitePath,
			SSLMode:        normalized.SSLMode,
			Remark:         normalized.Remark,
			Version:        1,
			CreatedBy:      scope.ActorID,
			CreatedAt:      now,
			UpdatedAt:      now,
		})
	}
	if len(entities) == 0 {
		return response.OK(ConnectionImportResponse{Created: 0, Skipped: skipped, Items: []ConnectionSimple{}}), nil
	}
	if err := r.repo.CreateMany(ctx, entities); err != nil {
		return zero, err
	}
	items := make([]ConnectionSimple, 0, len(entities))
	for _, entity := range entities {
		items = append(items, *ConnectionEntityToSimple(entity))
	}
	return response.OK(ConnectionImportResponse{Created: len(entities), Skipped: skipped, Items: items}), nil
}

func parseIDs(raw []string) ([]int64, error) {
	if len(raw) > 500 {
		return nil, apperrors.Invalid("一次最多导出 500 个连接")
	}
	ids := make([]int64, 0, len(raw))
	for _, item := range raw {
		item = strings.TrimSpace(item)
		if item == "" {
			return nil, apperrors.Invalid("连接主键无效")
		}
		id, err := strconv.ParseInt(item, 10, 64)
		if err != nil || id <= 0 {
			return nil, apperrors.Invalid("连接主键无效")
		}
		ids = append(ids, id)
	}
	return ids, nil
}

func uniqueIDs(ids []int64) []int64 {
	seen := map[int64]struct{}{}
	out := make([]int64, 0, len(ids))
	for _, id := range ids {
		if _, ok := seen[id]; ok {
			continue
		}
		seen[id] = struct{}{}
		out = append(out, id)
	}
	return out
}

func uniqueName(base string, used map[string]bool) string {
	base = strings.TrimSpace(base)
	if base == "" {
		base = "导入连接"
	}
	if !used[base] {
		return base
	}
	for i := 2; i < 1000; i++ {
		suffix := fmt.Sprintf(" (%d)", i)
		candidate := base + suffix
		if len([]rune(candidate)) > 80 {
			keep := 80 - len([]rune(suffix))
			if keep < 1 {
				keep = 1
			}
			runes := []rune(base)
			if keep > len(runes) {
				keep = len(runes)
			}
			candidate = string(runes[:keep]) + suffix
		}
		if !used[candidate] {
			return candidate
		}
	}
	return ""
}
