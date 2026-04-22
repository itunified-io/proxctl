package config

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"reflect"

	"gopkg.in/yaml.v3"
)

// LoadOptions tunes loader behaviour.
type LoadOptions struct {
	// SkipSecrets disables ${…} placeholder expansion.
	SkipSecrets bool
	// SkipValidate disables cross-field validation.
	SkipValidate bool
	// Resolver configures secret resolution.
	Resolver ResolverOpts
}

// Load reads an env manifest at path, applies profile extends, resolves $ref
// pointers, expands secret placeholders, and runs validation.
func Load(path string) (*Env, error) {
	return LoadWithOptions(path, LoadOptions{})
}

// LoadWithOptions is Load with explicit knobs.
func LoadWithOptions(path string, opts LoadOptions) (*Env, error) {
	path, err := filepath.Abs(path)
	if err != nil {
		return nil, fmt.Errorf("resolve %q: %w", path, err)
	}
	env, err := readEnv(path)
	if err != nil {
		return nil, err
	}
	baseDir := filepath.Dir(path)

	if env.Extends != "" {
		if err := applyProfile(env); err != nil {
			return nil, fmt.Errorf("apply profile %q: %w", env.Extends, err)
		}
	}

	seen := map[string]bool{path: true}
	if err := resolveRefs(env, baseDir, seen); err != nil {
		return nil, err
	}

	if !opts.SkipSecrets {
		if opts.Resolver.EnvName == "" {
			opts.Resolver.EnvName = env.Metadata.Name
		}
		res := ResolvePlaceholders(env, opts.Resolver)
		if len(res.Errors) > 0 {
			return nil, errors.Join(res.Errors...)
		}
	}

	if !opts.SkipValidate {
		if err := Validate(env); err != nil {
			return nil, err
		}
	}
	return env, nil
}

// readEnv reads + unmarshals one env file.
func readEnv(path string) (*Env, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read %q: %w", path, err)
	}
	var env Env
	if err := yaml.Unmarshal(b, &env); err != nil {
		return nil, fmt.Errorf("parse %q: %w", path, err)
	}
	return &env, nil
}

// applyProfile loads the embedded profile and deep-merges its fields into env
// for any field left unset by the concrete env.
func applyProfile(env *Env) error {
	raw, err := LoadProfile(env.Extends)
	if err != nil {
		return err
	}
	var base Env
	if err := yaml.Unmarshal(raw, &base); err != nil {
		return fmt.Errorf("parse profile %q: %w", env.Extends, err)
	}
	mergeEnv(env, &base)
	return nil
}

// mergeEnv copies fields from base into dst where dst has zero values.
func mergeEnv(dst, base *Env) {
	if dst.Version == "" {
		dst.Version = base.Version
	}
	if dst.Kind == "" {
		dst.Kind = base.Kind
	}
	if dst.Metadata.Description == "" {
		dst.Metadata.Description = base.Metadata.Description
	}
	// Cluster: if dst has no cluster ref/inline, use base's.
	if dst.Spec.Cluster == nil && base.Spec.Cluster != nil {
		dst.Spec.Cluster = base.Spec.Cluster
	} else if dst.Spec.Cluster != nil && base.Spec.Cluster != nil &&
		dst.Spec.Cluster.Inline != nil && base.Spec.Cluster.Inline != nil {
		// Merge cluster type default from base.
		if dst.Spec.Cluster.Inline.Type == "" {
			dst.Spec.Cluster.Inline.Type = base.Spec.Cluster.Inline.Type
		}
	}
}

// resolveRefs walks Env.Spec.* Ref fields; for each with a non-empty Ref,
// reads the target file relative to baseDir and populates Value.
// Circular references are detected via the seen set.
func resolveRefs(env *Env, baseDir string, seen map[string]bool) error {
	if err := resolveRef(&env.Spec.Hypervisor, baseDir, seen); err != nil {
		return fmt.Errorf("resolve hypervisor: %w", err)
	}
	if err := resolveRef(&env.Spec.Networks, baseDir, seen); err != nil {
		return fmt.Errorf("resolve networks: %w", err)
	}
	if err := resolveRef(&env.Spec.StorageClasses, baseDir, seen); err != nil {
		return fmt.Errorf("resolve storage_classes: %w", err)
	}
	if env.Spec.Linux != nil {
		if err := resolveRef(env.Spec.Linux, baseDir, seen); err != nil {
			return fmt.Errorf("resolve linux: %w", err)
		}
	}
	if env.Spec.Cluster != nil {
		if err := resolveRef(env.Spec.Cluster, baseDir, seen); err != nil {
			return fmt.Errorf("resolve cluster: %w", err)
		}
	}
	for i := range env.Spec.Databases {
		if err := resolveRef(&env.Spec.Databases[i], baseDir, seen); err != nil {
			return fmt.Errorf("resolve databases[%d]: %w", i, err)
		}
	}
	return nil
}

// resolveRef handles one generic Ref[T]: if Ref is set, read+decode the file.
func resolveRef[T any](r *Ref[T], baseDir string, seen map[string]bool) error {
	if r.Ref == "" {
		// Inline path: copy Inline → Value so callers only read Value.
		if r.Inline != nil && r.Value == nil {
			v := *r.Inline
			r.Value = &v
		}
		return nil
	}
	path := r.Ref
	if !filepath.IsAbs(path) {
		path = filepath.Join(baseDir, path)
	}
	path, err := filepath.Abs(path)
	if err != nil {
		return err
	}
	if seen[path] {
		return fmt.Errorf("circular $ref: %s", path)
	}
	seen[path] = true
	b, err := os.ReadFile(path)
	if err != nil {
		return fmt.Errorf("read %q: %w", path, err)
	}
	var t T
	if err := yaml.Unmarshal(b, &t); err != nil {
		return fmt.Errorf("parse %q: %w", path, err)
	}
	r.Value = &t
	_ = reflect.TypeOf(t)
	return nil
}
