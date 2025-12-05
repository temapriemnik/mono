package clickhouse

import (
	"context"
	"nats-clickhouse-service/internal/models"

	"github.com/ClickHouse/clickhouse-go/v2"
	"github.com/ClickHouse/clickhouse-go/v2/lib/driver"
)

type Client struct {
	conn driver.Conn
}

func NewClient(url, database string) (*Client, error) {
	conn, err := clickhouse.Open(&clickhouse.Options{
		Addr: []string{url},
		Auth: clickhouse.Auth{
			Database: database,
			Username: "default",
        	Password: "password",
		},
	})
	if err != nil {
		return nil, err
	}
	return &Client{conn: conn}, nil
}

func (c *Client) CreateTable() error {
	ctx := context.Background()
	return c.conn.Exec(ctx, `
		CREATE TABLE IF NOT EXISTS vacancies (
			id Int32,
			city String,
			name String,
			required_experience String,
			description String,
			salary_from Int32,
			salary_to Int32,
			published_at DateTime64(3, 'UTC'),
			status String,
			skills String
		) ENGINE = ReplacingMergeTree()
		ORDER BY id
	`)
}

func (c *Client) InsertVacancy(vacancy *models.Vacancy) error {
	ctx := context.Background()
	batch, err := c.conn.PrepareBatch(ctx, "INSERT INTO vacancies")
	if err != nil {
		return err
	}

	if err := batch.Append(
		vacancy.ID,
		vacancy.City,
		vacancy.Name,
		vacancy.RequiredExperience,
		vacancy.Description,
		vacancy.SalaryFrom,
		vacancy.SalaryTo,
		vacancy.PublishedAt,
		vacancy.Status,
		vacancy.Skills,
	); err != nil {
		return err
	}

	return batch.Send()
}

func (c *Client) Close() error {
	return c.conn.Close()
}
