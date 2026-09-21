package main

import (
	"backend/internal/config"
	"backend/internal/db"
	"backend/internal/moderation"
	"flag"
	"log"
)

func main() {
	name := flag.String("account", "", "existing staff account (operator recovery only)")
	revoke := flag.Bool("revoke", false, "disable staff access")
	flag.Parse()
	if *name == "" {
		log.Fatal("usage: admin -account ACCOUNT [-revoke]")
	}
	cfg, _, err := config.LoadLocalDev("configs/config.yaml")
	if err != nil {
		log.Fatal(err)
	}
	database, err := db.New(cfg.Database)
	if err != nil {
		log.Fatal(err)
	}
	err = moderation.RecoverStaff(database, *name, *revoke)
	if err != nil {
		log.Fatal(err)
	}
	log.Printf("staff %s recovery completed; manage members in the review website", *name)
}
