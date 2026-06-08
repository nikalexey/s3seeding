// Package config loads the benchmark configuration from JSON and applies
// secret overrides from environment variables.
package config

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"strings"
)

type Storage struct {
	Name               string `json:"name"`
	Endpoint           string `json:"endpoint"`
	Bucket             string `json:"bucket"`
	AccessKey          string `json:"access_key"`
	SecretKey          string `json:"secret_key"`
	PathStyle          bool   `json:"path_style"`
	InsecureSkipVerify bool   `json:"insecure_skip_verify"`
}

// SizeRange is an inclusive object-size range in kilobytes.
type SizeRange struct {
	MinKB int64 `json:"min_kb"`
	MaxKB int64 `json:"max_kb"`
}

// Seed controls how the seed command generates random objects: the share of
// each size class (percentages should sum to ~100) and the size range per class.
type Seed struct {
	SmallPct  float64   `json:"small_pct"`
	MediumPct float64   `json:"medium_pct"`
	LargePct  float64   `json:"large_pct"`
	Small     SizeRange `json:"small"`
	Medium    SizeRange `json:"medium"`
	Large     SizeRange `json:"large"`
}

type Config struct {
	PartSizeMB int       `json:"part_size_mb"`
	Storages   []Storage `json:"storages"`
	Seed       Seed      `json:"seed"`
}

func Load(path string) (*Config, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read config: %w", err)
	}
	var c Config
	if err := json.Unmarshal(raw, &c); err != nil {
		return nil, fmt.Errorf("parse config %s: %w", path, err)
	}
	c.applyDefaults()
	c.applyEnv()
	if err := c.validate(); err != nil {
		return nil, err
	}
	return &c, nil
}

func (c *Config) applyDefaults() {
	if c.PartSizeMB <= 0 {
		c.PartSizeMB = 64
	}
	if c.Seed.SmallPct == 0 && c.Seed.MediumPct == 0 && c.Seed.LargePct == 0 {
		c.Seed.SmallPct = 100
	}
	if c.Seed.Small == (SizeRange{}) {
		c.Seed.Small = SizeRange{MinKB: 1, MaxKB: 100}
	}
	if c.Seed.Medium == (SizeRange{}) {
		c.Seed.Medium = SizeRange{MinKB: 100, MaxKB: 10240}
	}
	if c.Seed.Large == (SizeRange{}) {
		c.Seed.Large = SizeRange{MinKB: 10240, MaxKB: 512000}
	}
}

func (c *Config) applyEnv() {
	for i := range c.Storages {
		base := "S3BENCH_" + sanitizeEnv(c.Storages[i].Name)
		if v, ok := os.LookupEnv(base + "_ACCESS_KEY"); ok {
			c.Storages[i].AccessKey = v
		}
		if v, ok := os.LookupEnv(base + "_SECRET_KEY"); ok {
			c.Storages[i].SecretKey = v
		}
		if v, ok := os.LookupEnv(base + "_ENDPOINT"); ok {
			c.Storages[i].Endpoint = v
		}
		if v, ok := os.LookupEnv(base + "_BUCKET"); ok {
			c.Storages[i].Bucket = v
		}
	}
}

func sanitizeEnv(name string) string {
	var b strings.Builder
	for _, r := range strings.ToUpper(name) {
		if (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9') {
			b.WriteRune(r)
		} else {
			b.WriteByte('_')
		}
	}
	return b.String()
}

func (c *Config) validate() error {
	if len(c.Storages) == 0 {
		return errors.New("config: no storages defined")
	}
	seen := map[string]bool{}
	for _, s := range c.Storages {
		switch {
		case s.Name == "":
			return errors.New("config: storage has an empty name")
		case seen[s.Name]:
			return fmt.Errorf("config: duplicate storage name %q", s.Name)
		case s.Endpoint == "":
			return fmt.Errorf("config: storage %q has an empty endpoint", s.Name)
		case s.Bucket == "":
			return fmt.Errorf("config: storage %q has an empty bucket", s.Name)
		case s.AccessKey == "" || s.SecretKey == "":
			return fmt.Errorf("config: storage %q has no access/secret key (set it in the config or via S3BENCH_%s_ACCESS_KEY/_SECRET_KEY)", s.Name, sanitizeEnv(s.Name))
		}
		seen[s.Name] = true
	}
	if c.PartSizeMB < 5 {
		return fmt.Errorf("config: part_size_mb must be >= 5 (minimum S3 multipart part size), got %d", c.PartSizeMB)
	}
	if c.Seed.SmallPct < 0 || c.Seed.MediumPct < 0 || c.Seed.LargePct < 0 {
		return errors.New("config: seed percentages must be non-negative")
	}
	if c.Seed.SmallPct+c.Seed.MediumPct+c.Seed.LargePct <= 0 {
		return errors.New("config: seed percentages must sum to more than 0")
	}
	for _, r := range []struct {
		name string
		sr   SizeRange
	}{{"small", c.Seed.Small}, {"medium", c.Seed.Medium}, {"large", c.Seed.Large}} {
		if r.sr.MinKB < 0 || r.sr.MaxKB < r.sr.MinKB {
			return fmt.Errorf("config: seed %s range invalid (min_kb=%d max_kb=%d)", r.name, r.sr.MinKB, r.sr.MaxKB)
		}
	}
	return nil
}
