package catalog

type CatalogSearch struct {
	ConnectionID int64  `json:"connection_id,string"`
	Schema       string `json:"schema"`
	Table        string `json:"table"`
	Name         string `json:"name"`
	Kind         string `json:"kind"`
}

type CatalogSchema struct {
	Name string `json:"name"`
}

type CatalogTable struct {
	Schema string `json:"schema"`
	Name   string `json:"name"`
	Type   string `json:"type"`
}

type CatalogColumn struct {
	Name     string `json:"name"`
	Type     string `json:"type"`
	Nullable bool   `json:"nullable"`
	Default  string `json:"default,omitempty"`
	Primary  bool   `json:"primary"`
	Extra    string `json:"extra,omitempty"`
}

type CatalogIndex struct {
	Name    string `json:"name"`
	Unique  bool   `json:"unique"`
	Columns string `json:"columns"`
}

type CatalogSuggestColumn struct {
	Name string `json:"name"`
	Type string `json:"type"`
}

type CatalogSuggestTable struct {
	Schema  string                 `json:"schema"`
	Name    string                 `json:"name"`
	Type    string                 `json:"type"`
	Columns []CatalogSuggestColumn `json:"columns"`
}

type CatalogSuggestResponse struct {
	Driver    string                `json:"driver"`
	Schema    string                `json:"schema"`
	Schemas   []CatalogSchema       `json:"schemas"`
	Tables    []CatalogSuggestTable `json:"tables"`
	Truncated bool                  `json:"truncated"`
}

type CatalogObject struct {
	Schema string `json:"schema"`
	Name   string `json:"name"`
	Kind   string `json:"kind"`
	Detail string `json:"detail,omitempty"`
}

type CatalogObjectsResponse struct {
	Driver    string          `json:"driver"`
	Schema    string          `json:"schema"`
	Objects   []CatalogObject `json:"objects"`
	Supported bool            `json:"supported"`
}

type CatalogDefinitionResponse struct {
	Driver   string `json:"driver"`
	Schema   string `json:"schema"`
	Name     string `json:"name"`
	Kind     string `json:"kind"`
	SQL      string `json:"sql"`
	Editable bool   `json:"editable"`
	Note     string `json:"note,omitempty"`
}

type CatalogColumnDraft struct {
	Name     string `json:"name"`
	Type     string `json:"type"`
	Nullable bool   `json:"nullable"`
	Primary  bool   `json:"primary"`
	Identity bool   `json:"identity"`
	Default  string `json:"default"`
}

type CatalogParamDraft struct {
	Name string `json:"name"`
	Type string `json:"type"`
	Mode string `json:"mode"`
}

type CatalogPreviewRequest struct {
	Driver    string               `json:"driver"`
	Schema    string               `json:"schema"`
	Kind      string               `json:"kind"`
	Name      string               `json:"name"`
	Replace   bool                 `json:"replace"`
	Columns   []CatalogColumnDraft `json:"columns"`
	SelectSQL string               `json:"select_sql"`
	Params    []CatalogParamDraft  `json:"params"`
	Returns   string               `json:"returns"`
	Language  string               `json:"language"`
	Body      string               `json:"body"`
}

type CatalogPreviewResponse struct {
	Driver string `json:"driver"`
	SQL    string `json:"sql"`
}

type CatalogApplyRequest struct {
	ConnectionID int64  `json:"connection_id,string"`
	SQL          string `json:"sql"`
}

type CatalogApplyResponse struct {
	Driver     string `json:"driver"`
	Statements int    `json:"statements"`
	DurationMS int64  `json:"duration_ms"`
	RowCount   int64  `json:"row_count"`
}

type CatalogResponse struct {
	Driver  string          `json:"driver"`
	Schemas []CatalogSchema `json:"schemas"`
	Tables  []CatalogTable  `json:"tables"`
	Columns []CatalogColumn `json:"columns"`
	Indexes []CatalogIndex  `json:"indexes"`
}
