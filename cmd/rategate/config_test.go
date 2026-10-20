package main

import "testing"

func TestLoadConfig(t *testing.T) {
	for _, tt := range []struct {
		name  string
		env   map[string]string
		rate  int
		burst int
	}{
		{"defaults", map[string]string{}, 10, 10},
		{"custom", map[string]string{"RATE_LIMIT": "5", "RATE_BURST": "20"}, 5, 20},
		{"rate only", map[string]string{"RATE_LIMIT": "1"}, 1, 10},
		{"burst only", map[string]string{"RATE_BURST": "1000000"}, 10, 1000000},
	} {
		t.Run(tt.name, func(t *testing.T) {
			tt.env["UPSTREAM_URL"] = "http://localhost:9000"
			cfg, err := loadConfig(func(name string) string { return tt.env[name] })
			if err != nil {
				t.Fatal(err)
			}
			if cfg.rate != tt.rate || cfg.burst != tt.burst || cfg.upstream != tt.env["UPSTREAM_URL"] {
				t.Fatalf("unexpected config: %+v", cfg)
			}
		})
	}
}

func TestLoadConfigRejectsInvalidLimits(t *testing.T) {
	for _, name := range []string{"RATE_LIMIT", "RATE_BURST"} {
		for _, raw := range []string{"0", "-1", "1.5", "NaN", "abc", "1000001", "999999999999999999999999", " "} {
			t.Run(name+"/"+raw, func(t *testing.T) {
				_, err := loadConfig(func(key string) string {
					if key == name {
						return raw
					}
					return ""
				})
				if err == nil {
					t.Fatalf("accepted %s=%q", name, raw)
				}
			})
		}
	}
}
