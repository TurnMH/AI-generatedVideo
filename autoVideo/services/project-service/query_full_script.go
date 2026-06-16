package main

import (
	"fmt"
	"log"

	"gorm.io/driver/postgres"
	"gorm.io/gorm"
)

type Episode struct {
	ID            uint64 `gorm:"primaryKey"`
	ProjectID     uint64
	EpisodeNumber int
	ScriptExcerpt string
}

func main() {
	dsn := "host=localhost port=5432 user=postgres password=postgres dbname=project_db sslmode=disable TimeZone=Asia/Shanghai"
	db, err := gorm.Open(postgres.Open(dsn), &gorm.Config{})
	if err != nil {
		log.Fatalf("failed to connect database: %v", err)
	}

	var ep Episode
	err = db.Where("project_id = ? AND episode_number = ?", 193, 1).First(&ep).Error
	if err != nil {
		log.Fatalf("failed to query episode: %v", err)
	}

	fmt.Printf("ScriptExcerpt:\n%s\n", ep.ScriptExcerpt)
}
