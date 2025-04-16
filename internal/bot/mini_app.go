// This file should be placed in internal/bot/mini_app.go

package bot

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"food_delivery/internal/db"
	"food_delivery/internal/models"
	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
	"log"
)

// MiniAppOrderData represents the data structure received from the Mini App
type MiniAppOrderData struct {
	UserTelegramId int64 `json:"userTelegramId"`
	Items          []struct {
		MealId   string `json:"mealId"`
		Quantity int    `json:"quantity"`
	} `json:"items"`
	ContactInfo struct {
		PhoneNumber string `json:"phoneNumber"`
		Address     string `json:"address"`
		Comment     string `json:"comment"`
	} `json:"contactInfo"`
}

// HandleMiniAppData processes data received from the Telegram Mini App
func HandleMiniAppData(bot *tgbotapi.BotAPI, update tgbotapi.Update, database *sql.DB) {
	// Extract data from the WebApp
	webAppData := update.Message.WebAppData
	if webAppData == nil {
		log.Println("No web app data received")
		return
	}

	// Parse the data
	var orderData MiniAppOrderData
	err := json.Unmarshal([]byte(webAppData.Data), &orderData)
	if err != nil {
		log.Printf("Error parsing mini app data: %v", err)
		msg := tgbotapi.NewMessage(update.Message.Chat.ID, "❌ Error processing your order. Please try again.")
		bot.Send(msg)
		return
	}

	// Log the received data
	log.Printf("Received order from Mini App: %+v", orderData)

	// Create an order from the received data
	order := models.Order{
		User: models.User{
			TelegramID: orderData.UserTelegramId,
		},
		Location: orderData.ContactInfo.Address,
		Comment:  orderData.ContactInfo.Comment,
	}

	// Add order items
	for _, item := range orderData.Items {
		// Get meal details from the database
		var meal models.Meal
		err := database.QueryRow(
			"SELECT id, name, price FROM meals WHERE id = $1",
			item.MealId,
		).Scan(&meal.ID, &meal.Name, &meal.Price)

		if err != nil {
			log.Printf("Error retrieving meal %s: %v", item.MealId, err)
			continue
		}

		// Add meal to order
		order.OrderMeal = append(order.OrderMeal, models.OrderMeal{
			Meal:     meal,
			Quantity: item.Quantity,
		})
	}

	// Save the order
	err = db.SaveOrder(database, &order)
	if err != nil {
		log.Printf("Error saving order: %v", err)
		msg := tgbotapi.NewMessage(update.Message.Chat.ID, "❌ Failed to save your order. Please try again.")
		bot.Send(msg)
		return
	}

	// Send confirmation to user
	msg := tgbotapi.NewMessage(update.Message.Chat.ID, "✅ Your order has been placed successfully! We'll process it and deliver to your address soon.")
	bot.Send(msg)

	// Send notification to admin group
	// Calculate total price
	totalPrice := 0
	orderSummary := fmt.Sprintf("🛒 *New Order via Mini App*\n👤 *Customer:* %s\n📞 *Phone:* %s\n📍 *Address:* %s\n\n",
		update.Message.From.FirstName,
		orderData.ContactInfo.PhoneNumber,
		orderData.ContactInfo.Address)

	for _, item := range order.OrderMeal {
		itemTotal := item.Quantity * item.Meal.Price
		totalPrice += itemTotal
		orderSummary += fmt.Sprintf("- %d x %s: %d UZS\n", item.Quantity, item.Meal.Name, itemTotal)
	}

	if orderData.ContactInfo.Comment != "" {
		orderSummary += fmt.Sprintf("\n💬 *Comment:* %s", orderData.ContactInfo.Comment)
	}

	orderSummary += fmt.Sprintf("\n💰 *Total Price:* %d UZS", totalPrice)

	adminMsg := tgbotapi.NewMessage(adminGroupID, orderSummary)
	adminMsg.ParseMode = "Markdown"
	bot.Send(adminMsg)
}
