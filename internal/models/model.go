package models

import (
	"database/sql"
	"time"
)

// User represents a registered user in the system
type User struct {
	ID          string    `json:"id"` // UUID
	Name        string    `json:"name"`
	PhoneNumber string    `json:"phone_number"`
	Role        string    `json:"role"`
	TelegramID  int64     `json:"telegram_id"`
	CreatedAt   time.Time `json:"created_at"`
}

type Meal struct {
	ID    string `json:"id"`    // UUID
	Name  string `json:"name"`  // Meal name
	Price int    `json:"price"` // Meal price
}

type OrderMeal struct {
	Meal     Meal `json:"meal"`
	Quantity int  `json:"quantity"`
}

type Order struct {
	ID        string
	User      User
	OrderMeal []OrderMeal
	Comment   string
}

func (o *Order) Save(db *sql.DB) error {
	tx, err := db.Begin()
	if err != nil {
		return err
	}

	// Insert order
	orderQuery := "INSERT INTO orders (id, user_id, comment) VALUES (?, ?, ?)"
	_, err = tx.Exec(orderQuery, o.ID, o.User.ID, o.Comment)
	if err != nil {
		tx.Rollback()
		return err
	}

	// Insert order meals
	mealQuery := "INSERT INTO order_meals (order_id, meal_id, quantity) VALUES (?, ?, ?)"
	for _, om := range o.OrderMeal {
		_, err := tx.Exec(mealQuery, o.ID, om.Meal.ID, om.Quantity)
		if err != nil {
			tx.Rollback()
			return err
		}
	}

	return tx.Commit()
}
