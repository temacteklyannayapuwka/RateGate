package main

import (
	"fmt"
	"strconv"
	"time"
)

const (
	defaultRate            = 10
	defaultBurst           = 10
	defaultUpstreamTimeout = 5 * time.Second
)

type config struct {
	upstream        string
	rate            int
	burst           int
	upstreamTimeout time.Duration
}

func loadConfig(getenv func(string) string) (config, error) {
	cfg := config{upstream: getenv("UPSTREAM_URL"), rate: defaultRate, burst: defaultBurst, upstreamTimeout: defaultUpstreamTimeout}
	for _, setting := range []struct {
		name   string
		target *int
	}{
		{"RATE_LIMIT", &cfg.rate},
		{"RATE_BURST", &cfg.burst},
	} {
		raw := getenv(setting.name)
		if raw == "" {
			continue
		}
		value, err := strconv.Atoi(raw)
		if err != nil || value < 1 || value > 1000000 {
			return config{}, fmt.Errorf("%s must be an integer between 1 and 1000000", setting.name)
		}
		*setting.target = value
	}
	if raw := getenv("UPSTREAM_TIMEOUT"); raw != "" {
		value, err := time.ParseDuration(raw)
		if err != nil || value <= 0 || value > time.Minute {
			return config{}, fmt.Errorf("UPSTREAM_TIMEOUT must be a positive duration no greater than 1m")
		}
		cfg.upstreamTimeout = value
	}
	return cfg, nil
}
