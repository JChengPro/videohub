package main

import (
	"backend/internal/cache"
	"backend/internal/config"
	"backend/internal/db"
	"backend/internal/mq"
	"backend/internal/observability"
	"backend/internal/router"
	"fmt"
	"log"
)

func main() {
	cfg, usedDefault, err := config.LoadLocalDev("configs/config.yaml")
	if err != nil {
		log.Fatalf("load config failed: %v", err)
	}
	log.Printf("config loaded, usedDefault=%v, serverPort=%d", usedDefault, cfg.Server.Port)

	apiPprof, err := observability.NewPprofServer(
		"api",
		cfg.Observability.Pprof.Enabled,
		cfg.Observability.Pprof.APIAddr,
	)
	if err != nil {
		log.Fatalf("start api pprof failed: %v", err)
	}
	defer apiPprof.Close()

	sqlDB, err := db.New(cfg.Database)
	if err != nil {
		log.Fatalf("connect mysql failed: %v", err)
	}
	if err := db.AutoMigrate(sqlDB); err != nil {
		log.Fatalf("auto migrate failed: %v", err)
	}
	redisClient, err := cache.New(cfg.Redis)
	if err != nil {
		log.Printf("redis unavailable, cache disadled: %v", err)
		redisClient = nil
	}
	rabbit, err := mq.NewRabbitMQ(cfg.RabbitMQ)
	if err != nil {
		log.Printf("rabbitmq unavailable: %v", err)
		rabbit = nil
	} else {
		defer rabbit.Close()
	}
	r := router.New(sqlDB, redisClient, rabbit)

	addr := fmt.Sprintf(":%d", cfg.Server.Port)
	log.Printf("backend listening on: %s", addr)
	if err := r.Run(addr); err != nil {
		log.Fatalf("server stopped: %v", err)
	}
}
