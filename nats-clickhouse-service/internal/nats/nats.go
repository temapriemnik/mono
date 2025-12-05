package nats

import (
	"encoding/json"
	"log"
	"nats-clickhouse-service/internal/models"
	"github.com/nats-io/nats.go"
)

type Consumer struct {
	nc *nats.Conn
}

func NewConsumer(url string) (*Consumer, error) {
	nc, err := nats.Connect(url)
	if err != nil {
		return nil, err
	}
	return &Consumer{nc: nc}, nil
}

func (c *Consumer) Subscribe(subject string, handler func(*models.Vacancy)) error {
	_, err := c.nc.Subscribe(subject, func(msg *nats.Msg) {
		var vacancy models.Vacancy
		if err := json.Unmarshal(msg.Data, &vacancy); err != nil {
			log.Printf("Error unmarshaling message: %v", err)
			return
		}
		handler(&vacancy)
	})
	if err != nil {
		return err
	}
	return nil
}

func (c *Consumer) Close() {
	c.nc.Close()
}
