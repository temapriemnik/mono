package models

import (
	"time"
)

type Vacancy struct {
	ID                 int32     `json:"id"`
	City               string    `json:"city"`
	Name               string    `json:"name"`
	RequiredExperience string    `json:"required_experience"`
	Description        string    `json:"description"`
	SalaryFrom         int32     `json:"salary_from"`
	SalaryTo           int32     `json:"salary_to"`
	PublishedAt        time.Time `json:"published_at"`
	Status             string    `json:"status"`
	Skills             string    `json:"skills"`
}
