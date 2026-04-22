package config

import "errors"

// ExportJSONSchema emits JSON Schema documents for each apiVersion/kind
// into the target directory. Phase 1 stub.
//
// Phase 2 will use github.com/invopop/jsonschema to reflect these structs.
func ExportJSONSchema(_ string) error {
	return errors.New("config.ExportJSONSchema: not implemented yet (scaffold)")
}
