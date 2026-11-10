package catalog

type CatalogSearch struct {
	ConnectionID int64  `json:"connection_id,string"`
	Schema       string `json:"schema"`
	Table        string `json:"table"`
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

type CatalogResponse struct {
	Driver  string          `json:"driver"`
	Schemas []CatalogSchema `json:"schemas"`
	Tables  []CatalogTable  `json:"tables"`
	Columns []CatalogColumn `json:"columns"`
	Indexes []CatalogIndex  `json:"indexes"`
}
