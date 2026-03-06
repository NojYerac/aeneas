package data

import (
	"context"
)

type Example struct {
	ID          string `json:"id" db:"id"`
	Name        string `json:"name" db:"name"`
	Description string `json:"description" db:"description"`
}

type DataSource interface {
	GetExample(ctx context.Context, id string) (*Example, error)
}
