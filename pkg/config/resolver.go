package config

import (
	"fmt"
	"regexp"
)

// secretPattern matches ${kind:spec} placeholders.
//
//	${vault:secret/data/path#key}
//	${env:VAR}
//	${file:~/.ssh/id_ed25519.pub}
//	${gen:password:32}
//	${ref:layers.networks.public}
var secretPattern = regexp.MustCompile(`\$\{(vault|env|file|gen|ref):([^}]+)\}`)

// Placeholder represents a parsed ${...} reference.
type Placeholder struct {
	Kind string // vault | env | file | gen | ref
	Spec string // everything after the colon, before the }
	Raw  string // full original expression
}

// ParsePlaceholders scans a string and returns every ${kind:spec} it contains.
// Phase 1: syntax parse only. Phase 2 will add resolution against Vault/env/file/gen/ref.
func ParsePlaceholders(input string) []Placeholder {
	matches := secretPattern.FindAllStringSubmatch(input, -1)
	out := make([]Placeholder, 0, len(matches))
	for _, m := range matches {
		out = append(out, Placeholder{Kind: m[1], Spec: m[2], Raw: m[0]})
	}
	return out
}

// Resolve is the Phase 2 entry point that will expand every placeholder
// in `input` against the configured providers. Phase 1 stub.
func Resolve(_ string) (string, error) {
	return "", fmt.Errorf("config.Resolve: not implemented yet (scaffold)")
}
