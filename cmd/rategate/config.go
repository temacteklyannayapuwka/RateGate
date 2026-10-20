package main

import (
	"fmt"
	"strconv"
)

const (
	defaultRate  = 10
	defaultBurst = 10
)

type config struct {
	upstream string
	rate     int
	burst    int
}

func loadConfig(getenv func(string) string) (config, error) {
	cfg := config{upstream: getenv("UPSTREAM_URL"), rate: defaultRate, burst: defaultBurst}
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
	return cfg, nil
}
