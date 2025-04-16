package db

import (
	"database/sql"
	"fmt"
	"food_delivery/internal/models"
	"github.com/google/uuid"
)

func GetMealByName(db *sql.DB, name string) (*models.Meal, error) {
	var meal models.Meal
	err := db.QueryRow("SELECT id, name, price FROM meals WHERE name = ?", name).
		Scan(&meal.ID, &meal.Name, &meal.Price)
	if err != nil {
		return nil, err
	}
	return &meal, nil
}

// In internal/db/order.go, update the SaveOrder function:

func SaveOrder(db *sql.DB, order *models.Order) error {
	tx, err := db.Begin()
	if err != nil {
		return err
	}

	fmt.Printf("Order: %+v\n", order)
	fmt.Printf("User: %+v\n", order.User)
	fmt.Printf("User ID: %s\n", order.User.ID)

	// Retrieve user ID if it's missing
	if order.User.ID == "" {
		err = db.QueryRow("SELECT id FROM users WHERE telegram_id = $1", order.User.TelegramID).Scan(&order.User.ID)
		if err != nil {
			tx.Rollback()
			fmt.Println("Error: Failed to retrieve user ID!")
			return fmt.Errorf("user ID not found for Telegram ID: %d", order.User.TelegramID)
		}
		fmt.Printf("Retrieved User ID: %s\n", order.User.ID)
	}

	// Ensure order ID is not empty
	if order.ID == "" {
		order.ID = uuid.New().String()
	}

	// Insert order into orders table (now with location)
	_, err = tx.Exec("INSERT INTO orders (id, user_id, comment, location) VALUES ($1, $2, $3, $4);",
		order.ID, order.User.ID, order.Comment, order.Location)
	if err != nil {
		tx.Rollback()
		return err
	}

	// Insert each ordered meal
	for _, om := range order.OrderMeal {
		fmt.Println(om)

		if om.Meal.ID == "" {
			err := tx.QueryRow("SELECT id FROM meals WHERE name = $1", om.Meal.Name).Scan(&om.Meal.ID)
			if err != nil {
				tx.Rollback()
				return fmt.Errorf("failed to fetch meal ID for %s: %v", om.Meal.Name, err)
			}
		}

		_, err = tx.Exec("INSERT INTO order_meals (order_id, meal_id, quantity, price) VALUES ($1, $2, $3, $4)",
			order.ID, om.Meal.ID, om.Quantity, om.Meal.Price)
		if err != nil {
			tx.Rollback()
			return err
		}
	}

	// Commit transaction if everything is successful
	return tx.Commit()
}
