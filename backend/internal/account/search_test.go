package account

import (
	"context"
	"strings"
	"testing"

	gormmysql "gorm.io/driver/mysql"
	"gorm.io/gorm"
)

func TestSearchAccountsQueryKeepsRelevanceOrder(t *testing.T) {
	db, err := gorm.Open(gormmysql.New(gormmysql.Config{
		DSN:                       "root:password@tcp(127.0.0.1:3306)/videohub?charset=utf8mb4&parseTime=True&loc=Local",
		SkipInitializeWithVersion: true,
	}), &gorm.Config{
		DryRun:               true,
		DisableAutomaticPing: true,
	})
	if err != nil {
		t.Fatalf("open dry-run database: %v", err)
	}

	var users []Account
	tx := searchAccountsQuery(db, "test", 21, 0).Find(&users)
	if tx.Error != nil {
		t.Fatalf("build search query: %v", tx.Error)
	}
	sql := tx.Statement.SQL.String()
	if !strings.Contains(sql, "CASE WHEN LOWER(account_name) = LOWER(?)") {
		t.Fatalf("relevance order missing from SQL: %s", sql)
	}
	if !strings.Contains(sql, "LOWER(username) = LOWER(?)") {
		t.Fatalf("nickname relevance order missing from SQL: %s", sql)
	}
	if !strings.Contains(sql, "id ASC") {
		t.Fatalf("stable id order missing from SQL: %s", sql)
	}
}

func TestSearchValidation(t *testing.T) {
	service := NewService(nil, nil)
	if _, err := service.Search(context.Background(), " ", 20, 0); err == nil {
		t.Fatal("expected empty query to be rejected")
	}
	if _, err := service.Search(context.Background(), "test", 20, -1); err == nil {
		t.Fatal("expected negative offset to be rejected")
	}
}
