package snippet

import "time"

type SnippetRequest struct {
	ID          int64  `json:"id,string"`
	Name        string `json:"name"`
	Description string `json:"description"`
	Body        string `json:"body"`
	Driver      string `json:"driver"`
	Version     int64  `json:"version"`
}

type SnippetDeleteRequest struct {
	ID int64 `json:"id,string"`
}

type SnippetSearch struct {
	Keyword   string `json:"keyword"`
	Page      int    `json:"page"`
	PageSize  int    `json:"page_size"`
	Sort      string `json:"sort"`
	Direction string `json:"direction"`
}

type SnippetSimple struct {
	ID          int64     `json:"id,string"`
	Name        string    `json:"name"`
	Description string    `json:"description"`
	Driver      string    `json:"driver"`
	Version     int64     `json:"version"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
}

type SnippetResponse struct {
	ID          int64     `json:"id,string"`
	Name        string    `json:"name"`
	Description string    `json:"description"`
	Body        string    `json:"body"`
	Driver      string    `json:"driver"`
	Version     int64     `json:"version"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
}

type SnippetExportRequest struct {
	IDs []string `json:"ids"`
}

type SnippetPortable struct {
	Name        string `json:"name"`
	Description string `json:"description,omitempty"`
	Body        string `json:"body"`
	Driver      string `json:"driver,omitempty"`
}

type SnippetDocument struct {
	Format     string            `json:"format"`
	Version    int               `json:"version"`
	ExportedAt time.Time         `json:"exported_at"`
	Snippets   []SnippetPortable `json:"snippets"`
}

type SnippetImportRequest struct {
	Document   *SnippetDocument `json:"document"`
	OnConflict string           `json:"on_conflict"`
}

type SnippetImportResponse struct {
	Created int             `json:"created"`
	Skipped int             `json:"skipped"`
	Items   []SnippetSimple `json:"items"`
}
