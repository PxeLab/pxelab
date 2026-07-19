package models

import "time"

type ProfileScriptVersion struct {
	ID        uint      `json:"id" gorm:"primaryKey;autoIncrement"`
	ProfileID string    `json:"profile_id" gorm:"index;not null"`
	Content   string    `json:"content" gorm:"type:text;not null"`
	Checksum  string    `json:"checksum" gorm:"type:varchar(64);not null"`
	Comment   string    `json:"comment,omitempty"`
	CreatedAt time.Time `json:"created_at"`
}
