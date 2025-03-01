package db

import (
	"database/sql"
	"fmt"
	"food_delivery/internal/models"
	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
	"log"
)

// AddMeal inserts a new meal into the database
func AddMeal(db *sql.DB, meal *models.Meal) error {
	query := "INSERT INTO meals (id, name, price) VALUES (gen_random_uuid(), $1, $2)"
	_, err := db.Exec(query, meal.Name, meal.Price)
	if err != nil {
		return fmt.Errorf("failed to insert meal: %v", err)
	}
	return nil
}

// GetMeals retrieves all meals from the database
func GetMeals(db *sql.DB) ([]models.Meal, error) {
	rows, err := db.Query("SELECT id, name, price FROM meals ORDER BY name")
	if err != nil {
		return nil, fmt.Errorf("failed to fetch meals: %v", err)
	}
	defer rows.Close()

	var meals []models.Meal
	for rows.Next() {
		var meal models.Meal
		if err := rows.Scan(&meal.ID, &meal.Name, &meal.Price); err != nil {
			return nil, fmt.Errorf("failed to scan meal: %v", err)
		}
		meals = append(meals, meal)
	}

	return meals, nil
}

func IsMealName(bot *tgbotapi.BotAPI, update tgbotapi.Update, db *sql.DB, mealName string) bool {
	meals, err := GetMeals(db)
	if err != nil {
		log.Printf("Error fetching meals: %v", err)
		return false
	}

	for _, meal := range meals {
		if meal.Name == mealName {
			return true
		}
	}
	return false
}
