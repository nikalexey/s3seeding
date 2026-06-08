package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoadDefaultsAndEnvOverride(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "c.json")
	content := `{
	  "storages":[{"name":"s3-old","endpoint":"https://x","bucket":"b","access_key":"AK","secret_key":"SK"}]
	}`
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}

	t.Setenv("S3SEEDING_S3_OLD_SECRET_KEY", "ENVSECRET")

	cfg, err := Load(path)
	if err != nil {
		t.Fatal(err)
	}
	if got := cfg.Storages[0].SecretKey; got != "ENVSECRET" {
		t.Fatalf("env override secret = %q, want ENVSECRET", got)
	}
	if cfg.PartSizeMB != 64 {
		t.Fatalf("default part_size_mb = %d, want 64", cfg.PartSizeMB)
	}
	if cfg.Seed.SmallPct != 100 {
		t.Fatalf("default seed small_pct = %v, want 100", cfg.Seed.SmallPct)
	}
	if cfg.Seed.Small.MinKB != 1 || cfg.Seed.Small.MaxKB != 100 {
		t.Fatalf("default seed small range = %+v", cfg.Seed.Small)
	}
}

func TestValidateMissingKeys(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "c.json")
	content := `{
	  "storages":[{"name":"s3","endpoint":"https://x","bucket":"b"}]
	}`
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := Load(path); err == nil {
		t.Fatal("expected error due to missing keys")
	}
}

func TestSeedExplicitOverridesDefault(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "c.json")
	content := `{
	  "storages":[{"name":"s3","endpoint":"https://x","bucket":"b","access_key":"AK","secret_key":"SK"}],
	  "seed":{"small_pct":20,"medium_pct":30,"large_pct":50,"large":{"min_kb":1000,"max_kb":2000}}
	}`
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
	cfg, err := Load(path)
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Seed.MediumPct != 30 || cfg.Seed.LargePct != 50 {
		t.Fatalf("seed pct = %+v", cfg.Seed)
	}
	if cfg.Seed.Large.MinKB != 1000 || cfg.Seed.Large.MaxKB != 2000 {
		t.Fatalf("seed large range = %+v", cfg.Seed.Large)
	}
}
