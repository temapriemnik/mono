package main

import (
	"log"
	"nats-clickhouse-service/internal/clickhouse"
	"nats-clickhouse-service/internal/config"
	"nats-clickhouse-service/internal/models"
	"nats-clickhouse-service/internal/nats"
	"os"
	"os/signal"
	"syscall"
)

func main() {
	cfg := config.Load()

	clickhouseClient, err := clickhouse.NewClient(cfg.ClickHouseURL, cfg.Database)
	if err != nil {
		log.Fatal("Failed to connect to ClickHouse: ", err)
	}
	defer clickhouseClient.Close()

	if err := clickhouseClient.CreateTable(); err != nil {
		log.Fatal("Failed to create table: ", err)
	}

	natsConsumer, err := nats.NewConsumer(cfg.NATSURL)
	if err != nil {
		log.Fatal("Failed to connect to NATS: ", err)
	}
	defer natsConsumer.Close()

	err = natsConsumer.Subscribe("vacancies", func(vacancy *models.Vacancy) {
		if err := clickhouseClient.InsertVacancy(vacancy); err != nil {
			log.Printf("Failed to insert vacancy: %v", err)
		} else {
			log.Printf("Successfully inserted vacancy: %d", vacancy.ID)
		}
	})
	if err != nil {
		log.Fatal("Failed to subscribe to NATS: ", err)
	}

	log.Println("Service is running...")

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	log.Println("Shutting down...")
}