package packassets

import "embed"

const (
	// BuiltinJSONPattern matches the embedded built-in category files.
	BuiltinJSONPattern = "json/*.json"

	// SchemaPath is the embedded JSON schema path for pack category files.
	SchemaPath = "pack.schema.json"
)

// Files contains the built-in JSON pack assets and schema.
//
//go:embed pack.schema.json json/*.json
var Files embed.FS
