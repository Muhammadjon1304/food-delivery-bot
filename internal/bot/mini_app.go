// internal/bot/mini_app.go

package bot

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"food_delivery/internal/db"
	"food_delivery/internal/models"
	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
	"log"
	"net/http"
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

// SetupMiniAppWebhook sets up an HTTP handler for Mini App data
func SetupMiniAppWebhook(bot *tgbotapi.BotAPI, database *sql.DB) {
	http.HandleFunc("/mini-app-webhook", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "Only POST method is allowed", http.StatusMethodNotAllowed)
			return
		}

		// Parse the JSON request
		var orderData MiniAppOrderData
		decoder := json.NewDecoder(r.Body)
		if err := decoder.Decode(&orderData); err != nil {
			log.Printf("Error parsing mini app data: %v", err)
			http.Error(w, "Invalid request body", http.StatusBadRequest)
			return
		}

		// Log the received data
		log.Printf("Received order from Mini App: %+v", orderData)

		// Process the order (similar to your existing HandleMiniAppData function)
		processOrderFromMiniApp(bot, database, orderData)

		// Send a success response
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"status":"success"}`))
	})

	// Start the HTTP server
	go func() {
		log.Println("Starting Mini App webhook server on :8080")
		if err := http.ListenAndServe(":8080", nil); err != nil {
			log.Fatalf("Failed to start server: %v", err)
		}
	}()
}

// processOrderFromMiniApp processes the order data received from the Mini App
func processOrderFromMiniApp(bot *tgbotapi.BotAPI, database *sql.DB, orderData MiniAppOrderData) {
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
	err := db.SaveOrder(database, &order)
	if err != nil {
		log.Printf("Error saving order: %v", err)
		// Send error message to user
		msg := tgbotapi.NewMessage(orderData.UserTelegramId, "❌ Failed to save your order. Please try again.")
		bot.Send(msg)
		return
	}

	// Send confirmation to user
	msg := tgbotapi.NewMessage(orderData.UserTelegramId, "✅ Your order has been placed successfully! We'll process it and deliver to your address soon.")
	bot.Send(msg)

	// Calculate total price for admin notification
	totalPrice := 0
	orderSummary := fmt.Sprintf("🛒 *New Order via Mini App*\n👤 *Customer ID:* %d\n📞 *Phone:* %s\n📍 *Address:* %s\n\n",
		orderData.UserTelegramId,
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

	// Send notification to admin group
	adminMsg := tgbotapi.NewMessage(adminGroupID, orderSummary)
	adminMsg.ParseMode = "Markdown"
	bot.Send(adminMsg)
}

// AddMiniAppButton adds a Mini App button to an inline keyboard
func AddMiniAppButton(keyboard *tgbotapi.InlineKeyboardMarkup, text, url string) {
	row := tgbotapi.NewInlineKeyboardRow(
		tgbotapi.NewInlineKeyboardButtonURL(text, url),
	)
	keyboard.InlineKeyboard = append(keyboard.InlineKeyboard, row)
}
