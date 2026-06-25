package config

import "testing"

func TestApplyEnvOverridesServiceConnections(t *testing.T) {
	t.Setenv("DB_HOST", "prod-mysql")
	t.Setenv("DB_PORT", "3308")
	t.Setenv("DB_USER", "videohub")
	t.Setenv("DB_PASSWORD", "db-secret")
	t.Setenv("DB_NAME", "videohub_prod")
	t.Setenv("REDIS_HOST", "prod-redis")
	t.Setenv("REDIS_PORT", "6380")
	t.Setenv("REDIS_PASSWORD", "redis-secret")
	t.Setenv("RABBITMQ_HOST", "prod-rabbit")
	t.Setenv("RABBITMQ_PORT", "5673")
	t.Setenv("RABBITMQ_USERNAME", "worker")
	t.Setenv("RABBITMQ_PASSWORD", "rabbit-secret")
	t.Setenv("PPROF_ENABLED", "false")

	cfg := DefaultLocalConfig()
	applyEnv(&cfg)

	if cfg.Database.Host != "prod-mysql" || cfg.Database.Port != 3308 || cfg.Database.Password != "db-secret" {
		t.Fatalf("database environment overrides were not applied: %+v", cfg.Database)
	}
	if cfg.Redis.Host != "prod-redis" || cfg.Redis.Port != 6380 || cfg.Redis.Password != "redis-secret" {
		t.Fatalf("redis environment overrides were not applied: %+v", cfg.Redis)
	}
	if cfg.RabbitMQ.Host != "prod-rabbit" || cfg.RabbitMQ.Port != 5673 || cfg.RabbitMQ.Password != "rabbit-secret" {
		t.Fatalf("rabbitmq environment overrides were not applied: %+v", cfg.RabbitMQ)
	}
	if cfg.Observability.Pprof.Enabled {
		t.Fatal("PPROF_ENABLED=false was not applied")
	}
}
