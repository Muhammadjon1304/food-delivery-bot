package db

import (
	"database/sql"
	"fmt"
	"log"
	"os"

	_ "github.com/lib/pq"
)

// ConnectDB initializes the database connection
func ConnectDB() (*sql.DB, error) {
	dbURL := os.Getenv("DATABASE_URL")
	if dbURL == "" {
		log.Fatal("DATABASE_URL is missing in .env")
	}

	db, err := sql.Open("postgres", dbURL)
	if err != nil {
		return nil, fmt.Errorf("failed to open database: %v", err)
	}

	// Test the connection
	if err := db.Ping(); err != nil {
		return nil, fmt.Errorf("failed to ping database: %v", err)
	}

	fmt.Println("Connected to the database successfully!")
	return db, nil
}

// IsAdmin checks if a user is an admin
func IsAdmin(db *sql.DB, userID int64) (bool, error) {
	var role string
	query := "SELECT role FROM users WHERE telegram_id = $1"
	err := db.QueryRow(query, userID).Scan(&role)

	fmt.Println(role)
	if err != nil {
		if err == sql.ErrNoRows {
			return false, nil // User not found
		}
		return false, fmt.Errorf("error checking admin status: %v", err)
	}
	return role == "admin", nil
}
