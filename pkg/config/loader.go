package config

import "errors"

var (
	errEmpty             = errors.New("config: empty env manifest")
	errMissingAPIVersion = errors.New("config: missing apiVersion")
	errMissingKind       = errors.New("config: missing kind")
)

// Load reads an env.yaml manifest, resolves $refs + secrets, and returns the
// composed schema. Phase 1 stub.
func Load(_ string) (*Env, error) {
	return nil, errors.New("config.Load: not implemented yet (scaffold)")
}
