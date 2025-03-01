package User

import (
	"database/sql"
	"fmt"
	"food_delivery/internal/models"
)

// GetUserByTelegramID checks if a user exists
func GetUserByTelegramID(db *sql.DB, telegramID int64) (*models.User, error) {
	var user models.User
	query := "SELECT id, name, phone_number, role, telegram_id, created_at FROM users WHERE telegram_id = $1"
	err := db.QueryRow(query, telegramID).Scan(&user.ID, &user.Name, &user.PhoneNumber, &user.Role, &user.TelegramID, &user.CreatedAt)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, nil // User not found
		}
		return nil, fmt.Errorf("error checking user: %v", err)
	}
	return &user, nil
}

// RegisterUser adds a new user
func RegisterUser(db *sql.DB, user models.User) error {
	query := "INSERT INTO users (id, name, phone_number, role, telegram_id, created_at) VALUES (gen_random_uuid(), $1, $2, $3, $4, NOW())"
	_, err := db.Exec(query, user.Name, user.PhoneNumber, user.Role, user.TelegramID)
	if err != nil {
		return fmt.Errorf("failed to insert user: %v", err)
	}
	return nil
}
