package config

import (
	"errors"
	"flag"
	"fmt"
	"net"
	"strconv"
	"time"

	"github.com/caarlos0/env/v11"
)

type Config struct {
	Address        string `env:"RUN_ADDRESS"`
	DatabaseDSN    string `env:"DATABASE_URI"`
	AccrualAddress string `env:"ACCRUAL_SYSTEM_ADDRESS"`
	LogLevel       string
	Port           uint16
	// таймаут на get запрос в систему лояльости
	HttpClientTimeout time.Duration
	// количество воркеров для похода в систему лояльности
	WorkerCount uint8
	// с каким периодом обрабатываем запросы
	OrdersForProcessDuration time.Duration
	// даем ShutdownTimeout секунд на завершение работы сервера
	ShutdownTimeout time.Duration
	// с каким периодом делаем очистку
	CleanupAfterCrashDuration time.Duration
}

func NewConfig() (*Config, error) {
	cfg := Config{
		HttpClientTimeout:         5 * time.Second,
		WorkerCount:               10,
		OrdersForProcessDuration:  1 * time.Second,
		ShutdownTimeout:           5 * time.Second,
		CleanupAfterCrashDuration: 2 * time.Hour,
	}

	flag.StringVar(&cfg.Address, "a", "", "server address")
	flag.StringVar(&cfg.DatabaseDSN, "d", "", "database dsn")
	flag.StringVar(&cfg.AccrualAddress, "r", "", "accrual service address")
	flag.StringVar(&cfg.LogLevel, "l", "debug", "log level")
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

	portInt, err := strconv.ParseUint(port, 10, 16)
	if err != nil {
		return nil, fmt.Errorf("port required only digest: %w", err)
	}
	cfg.Port = uint16(portInt)

	if cfg.DatabaseDSN == "" {
		return nil, errors.New("dsn is required")
	}

	if cfg.AccrualAddress == "" {
		return nil, errors.New("accrual service address is required")
	}
	return &cfg, nil
}
