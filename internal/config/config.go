package config

import (
	"errors"
	"flag"
	"fmt"
	"net"
	"strconv"

	"github.com/caarlos0/env/v11"
)

type Config struct {
	Address        string `env:"RUN_ADDRESS"`
	DatabaseDSN    string `env:"DATABASE_URI"`
	AccuralAddress string `env:"ACCRUAL_SYSTEM_ADDRESS"`
	LogLevel       string
}

func NewConfig() (*Config, error) {
	var cfg Config

	flag.StringVar(&cfg.Address, "a", "", "server address")
	flag.StringVar(&cfg.DatabaseDSN, "d", "", "database dsn")
	flag.StringVar(&cfg.AccuralAddress, "r", "", "accural service address")
	flag.StringVar(&cfg.LogLevel, "l", "info", "log level")
	flag.Parse()

	if err := env.Parse(&cfg); err != nil {
		return nil, err
	}

	if cfg.Address == "" {
		return nil, errors.New("server address is required")
	}
	_, port, err := net.SplitHostPort(cfg.Address)
	if err != nil {
		return nil, fmt.Errorf("bad format, use host:port: %w", err)
	}

	_, err = strconv.ParseUint(port, 10, 32)
	if err != nil {
		return nil, fmt.Errorf("port required only digest: %w", err)
	}

	if cfg.DatabaseDSN == "" {
		return nil, errors.New("dsn is required")
	}
	return &cfg, nil
}
