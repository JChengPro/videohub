package config

import (
	"errors"
	"fmt"
	"os"
	"strconv"

	"gopkg.in/yaml.v3"
)

type Config struct {
	Server        ServerConfig        `yaml:"server"`
	Database      DatabaseConfig      `yaml:"database"`
	Redis         RedisConfig         `yaml:"redis"`
	RabbitMQ      RabbitMQConfig      `yaml:"rabbitmq"`
	Observability ObservabilityConfig `yaml:"observability"`
	Storage       StorageConfig       `yaml:"storage"`
}

type StorageConfig struct {
	Type string    `yaml:"type"`
	OSS  OSSConfig `yaml:"oss"`
}

type OSSConfig struct {
	Endpoint        string `yaml:"endpoint"`
	Region          string `yaml:"region"`
	BucketName      string `yaml:"bucket_name"`
	AccessKeyID     string `yaml:"-"`
	AccessKeySecret string `yaml:"-"`
}

type ServerConfig struct {
	Port int `yaml:"port"`
}

type DatabaseConfig struct {
	Host     string `yaml:"host"`
	Port     int    `yaml:"port"`
	User     string `yaml:"user"`
	Password string `yaml:"password"`
	DBName   string `yaml:"dbname"`
}

type RedisConfig struct {
	Host     string `yaml:"host"`
	Port     int    `yaml:"port"`
	Password string `yaml:"password"`
	DB       int    `yaml:"db"`
}

type RabbitMQConfig struct {
	Host     string `yaml:"host"`
	Port     int    `yaml:"port"`
	Username string `yaml:"username"`
	Password string `yaml:"password"`
}

type ObservabilityConfig struct {
	Pprof PprofConfig `yaml:"pprof"`
}

type PprofConfig struct {
	Enabled    bool   `yaml:"enabled"`
	APIAddr    string `yaml:"api_addr"`
	WorkerAddr string `yaml:"worker_addr"`
}

// 读取 yaml 文件
// 反序列化成 Config 结构体
func Load(filename string) (Config, error) {
	data, err := os.ReadFile(filename)
	if err != nil {
		return Config{}, fmt.Errorf("failed to read config file: %w", err)
	}

	var cfg Config
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return Config{}, fmt.Errorf("parse config %s: %w", filename, err)
	}

	// 使用环境变量覆盖存储配置，密钥不会写入YAML
	applyEnv(&cfg)

	return cfg, nil
}

// bool 表示是否使用了默认配置
// true  -> 使用了默认配置
// false -> 成功读取了配置文件
func LoadLocalDev(filename string) (Config, bool, error) {
	cfg, err := Load(filename)
	if err == nil {
		return cfg, false, nil
	}

	if errors.Is(err, os.ErrNotExist) {
		cfg := DefaultLocalConfig()
		applyEnv(&cfg)
		return cfg, true, nil
	}

	return Config{}, false, err
}

func DefaultLocalConfig() Config {
	return Config{
		Server: ServerConfig{
			Port: 8080,
		},
		Database: DatabaseConfig{
			Host:     "127.0.0.1",
			Port:     3307,
			User:     "root",
			Password: "123456",
			DBName:   "videohub",
		},
		Redis: RedisConfig{
			Host:     "127.0.0.1",
			Port:     6379,
			Password: "123456",
			DB:       0,
		},
		RabbitMQ: RabbitMQConfig{
			Host:     "127.0.0.1",
			Port:     5672,
			Username: "admin",
			Password: "password123",
		},
		Observability: ObservabilityConfig{
			Pprof: PprofConfig{
				Enabled:    true,
				APIAddr:    "127.0.0.1:6060",
				WorkerAddr: "127.0.0.1:6061",
			},
		},
	}
}

// 从 .env 中加载配置
func applyEnv(cfg *Config) {
	if value := os.Getenv("DB_HOST"); value != "" {
		cfg.Database.Host = value
	}
	if value := envInt("DB_PORT"); value != 0 {
		cfg.Database.Port = value
	}
	if value := os.Getenv("DB_USER"); value != "" {
		cfg.Database.User = value
	}
	if value := os.Getenv("DB_PASSWORD"); value != "" {
		cfg.Database.Password = value
	}
	if value := os.Getenv("DB_NAME"); value != "" {
		cfg.Database.DBName = value
	}

	if value := os.Getenv("REDIS_HOST"); value != "" {
		cfg.Redis.Host = value
	}
	if value := envInt("REDIS_PORT"); value != 0 {
		cfg.Redis.Port = value
	}
	if value := os.Getenv("REDIS_PASSWORD"); value != "" {
		cfg.Redis.Password = value
	}

	if value := os.Getenv("RABBITMQ_HOST"); value != "" {
		cfg.RabbitMQ.Host = value
	}
	if value := envInt("RABBITMQ_PORT"); value != 0 {
		cfg.RabbitMQ.Port = value
	}
	if value := os.Getenv("RABBITMQ_USERNAME"); value != "" {
		cfg.RabbitMQ.Username = value
	}
	if value := os.Getenv("RABBITMQ_PASSWORD"); value != "" {
		cfg.RabbitMQ.Password = value
	}

	if value := os.Getenv("PPROF_ENABLED"); value != "" {
		cfg.Observability.Pprof.Enabled = value == "true"
	}

	if value := os.Getenv("STORAGE_TYPE"); value != "" {
		cfg.Storage.Type = value
	}
	cfg.Storage.OSS.Endpoint = os.Getenv("OSS_ENDPOINT")
	cfg.Storage.OSS.Region = os.Getenv("OSS_REGION")
	cfg.Storage.OSS.BucketName = os.Getenv("OSS_BUCKET_NAME")
	cfg.Storage.OSS.AccessKeyID = os.Getenv("OSS_ACCESS_KEY_ID")
	cfg.Storage.OSS.AccessKeySecret = os.Getenv("OSS_ACCESS_KEY_SECRET")
}

func envInt(key string) int {
	value := os.Getenv(key)
	if value == "" {
		return 0
	}
	parsed, err := strconv.Atoi(value)
	if err != nil {
		return 0
	}
	return parsed
}
