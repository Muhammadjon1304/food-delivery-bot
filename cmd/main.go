package main

import (
	"fmt"
	"food_delivery/internal/db"
	"log"
	"os"

	"food_delivery/internal/bot"
	"github.com/joho/godotenv"
)

func main() {
	// Load environment variables
	if err := godotenv.Load(".env"); err != nil {
		log.Println("No .env file found, using system environment variables")
	}

	// Connect to database
	database, err := db.ConnectDB()
	if err != nil {
		log.Fatalf("Failed to connect to database: %v", err)
	}
	defer database.Close()

	// Start the Telegram bot
	botToken := os.Getenv("TELEGRAM_BOT_TOKEN")
	if botToken == "" {
		log.Fatal("TELEGRAM_BOT_TOKEN is missing in .env")
	}

	fmt.Println("Starting Telegram Bot...")
	bot.StartBot(botToken, database)
}
