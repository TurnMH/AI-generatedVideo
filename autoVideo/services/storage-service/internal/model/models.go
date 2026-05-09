package model

import (
	"time"

	"gorm.io/datatypes"
)

type File struct {
	ID            uint64         `gorm:"primaryKey;autoIncrement" json:"id"`
	UserID        uint64         `gorm:"index;not null" json:"user_id"`
	Bucket        string         `gorm:"size:64;not null" json:"bucket"`
	ObjectKey     string         `gorm:"size:512;not null;uniqueIndex" json:"object_key"`
	Filename      string         `gorm:"size:256" json:"filename"`
	ContentType   string         `gorm:"size:128" json:"content_type"`
	SizeBytes     int64          `json:"size_bytes"`
	CdnURL        string         `gorm:"type:text" json:"cdn_url"`
	Width         int            `json:"width,omitempty"`
	Height        int            `json:"height,omitempty"`
	Metadata      datatypes.JSON `gorm:"type:jsonb" json:"metadata"`
	ProjectID     *uint64        `gorm:"index" json:"project_id"`
	Category      string         `gorm:"size:50;default:other" json:"category"`
	IsCurrent     bool           `gorm:"default:true" json:"is_current"`
	VersionNumber int            `gorm:"default:1" json:"version_number"`
	RelatedID     *uint64        `json:"related_id"`
	RelatedType   string         `gorm:"size:50" json:"related_type"`
	Label         string         `gorm:"type:text" json:"label"`
	CreatedAt     time.Time      `json:"created_at"`
	UpdatedAt     time.Time      `json:"updated_at"`
}
