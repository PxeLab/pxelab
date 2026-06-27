package models

import (
	"encoding/json"
	"time"
)

type AnswerTemplate struct {
	ID             uint      `json:"id" gorm:"primaryKey"`
	Name           string    `json:"name" gorm:"not null"`
	Description    string    `json:"description"`
	Type           string    `json:"type" gorm:"not null"` // kickstart / preseed / subiquity / autoyast / autounattend
	Content        string    `json:"content" gorm:"type:text;not null"`
	CurrentVersion int       `json:"current_version" gorm:"default:1"`
	Variables      string    `json:"-" gorm:"type:text"` // JSON
	CreatedAt      time.Time `json:"created_at"`
	UpdatedAt      time.Time `json:"updated_at"`
}

type AnswerTemplateVersion struct {
	ID           uint      `json:"id" gorm:"primaryKey"`
	TemplateID   uint      `json:"template_id" gorm:"not null;index"`
	Version      int       `json:"version" gorm:"not null"`
	Content      string    `json:"content" gorm:"type:text;not null"`
	Description  string    `json:"description"`
	CreatedAt    time.Time `json:"created_at"`
}

func (t *AnswerTemplate) GetVariables() ([]string, error) {
	if t.Variables == "" {
		return nil, nil
	}
	var vars []string
	if err := json.Unmarshal([]byte(t.Variables), &vars); err != nil {
		return nil, err
	}
	return vars, nil
}

func (t *AnswerTemplate) SetVariables(vars []string) error {
	data, err := json.Marshal(vars)
	if err != nil {
		return err
	}
	t.Variables = string(data)
	return nil
}
