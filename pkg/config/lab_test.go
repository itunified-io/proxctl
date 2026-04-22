package config

import "testing"

func TestConfig_ValidateEmpty(t *testing.T) {
	var e *Env
	if err := e.Validate(); err == nil {
		t.Error("Validate on nil Env should fail")
	}
	if err := (&Env{}).Validate(); err == nil {
		t.Error("Validate on empty Env should fail (missing apiVersion/kind)")
	}
}

func TestConfig_ValidateHappy(t *testing.T) {
	e := &Env{APIVersion: APIVersionV1, Kind: "Env", Metadata: EnvMetadata{Name: "test"}}
	if err := e.Validate(); err != nil {
		t.Errorf("Validate on populated Env failed: %v", err)
	}
}

func TestConfig_ParsePlaceholders(t *testing.T) {
	in := `password: ${vault:secret/data/foo#key}
ssh: ${file:~/.ssh/id_ed25519.pub}
token: ${env:MY_TOK}
pw: ${gen:password:32}
ref: ${ref:layers.networks.public}`
	got := ParsePlaceholders(in)
	if len(got) != 5 {
		t.Fatalf("ParsePlaceholders returned %d, want 5 — %+v", len(got), got)
	}
	kinds := map[string]bool{}
	for _, p := range got {
		kinds[p.Kind] = true
	}
	for _, want := range []string{"vault", "file", "env", "gen", "ref"} {
		if !kinds[want] {
			t.Errorf("missing kind %q in %+v", want, got)
		}
	}
}
