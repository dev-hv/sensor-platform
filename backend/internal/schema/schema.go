package schema

// Schema represents the parsed schema.yaml structure.
type Schema struct {
	MandatoryColumns []ColumnDef  `yaml:"mandatory_columns" json:"mandatory_columns"`
	DynamicColumns   []DynamicCol `yaml:"dynamic_columns" json:"dynamic_columns"`
	TableName        string       `yaml:"table_name" json:"table_name"`
}

// ColumnDef defines a mandatory column (name + sql_type only).
type ColumnDef struct {
	Name    string `yaml:"name" json:"name"`
	SQLType string `yaml:"sql_type" json:"sql_type"`
}

// DynamicCol defines a data column with optional min/max validation.
type DynamicCol struct {
	Name     string   `yaml:"name" json:"name"`
	SQLType  string   `yaml:"sql_type" json:"sql_type"`
	MinValue *float64 `yaml:"min_value,omitempty" json:"min_value,omitempty"`
	MaxValue *float64 `yaml:"max_value,omitempty" json:"max_value,omitempty"`
}
