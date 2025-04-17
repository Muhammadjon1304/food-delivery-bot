package bot

import (
	"database/sql"
	"fmt"
	psDB "food_delivery/internal/db"
	"food_delivery/internal/models"
	users "food_delivery/internal/users"
	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
	"log"
	"strconv"
	"sync"
)

const adminGroupID = -4621007983

var (
	pendingOrders     = make(map[int64]models.Order) // Tracks ongoing orders
	orderMutex        = sync.Mutex{}                 // Prevents race conditions
	userSelectedMeals = make(map[int64]*models.Meal) // Tracks last selected meal for each user
)

var pendingMeals = make(map[int64]*models.Meal) // Telegram ID → Meal Struct
var mealMutex sync.Mutex

var availableMeals = map[string]models.Meal{
	"Makaron":  {Name: "Makaron", Price: 15000},
	"Mastava":  {Name: "Mastava", Price: 12000},
	"Osh":      {Name: "Osh", Price: 20000},
	"Shashlik": {Name: "Shashlik", Price: 25000},
	"Sho'rva":  {Name: "Sho'rva", Price: 14000},
}

// InitBot initializes the Telegram bot
func InitBot(botToken string) (*tgbotapi.BotAPI, error) {
	bot, err := tgbotapi.NewBotAPI(botToken)
	if err != nil {
		return nil, fmt.Errorf("failed to create bot: %v", err)
	}

	bot.Debug = true // Enable debugging
	fmt.Printf("Authorized on account %s\n", bot.Self.UserName)

	return bot, nil
}

// StartBot initializes and runs the Telegram bot
func StartBot(botToken string, db *sql.DB) {
	bot, err := tgbotapi.NewBotAPI(botToken)
	if err != nil {
		log.Fatalf("Failed to create bot: %v", err)
	}

	bot.Debug = true // Enable debugging
	fmt.Printf("Authorized on account %s\n", bot.Self.UserName)

	u := tgbotapi.NewUpdate(0)
	u.Timeout = 60
	updates := bot.GetUpdatesChan(u)

	for update := range updates {
		if update.Message == nil {
			continue
		}

		telegramID := update.Message.From.ID
		text := update.Message.Text

		// ✅ Handle phone number contact separately
		if update.Message.Contact != nil {
			HandleContact(bot, update, db)
			continue
		}

		if update.Message.Location != nil {
			handleLocation(bot, update, db)
			continue
		}

		orderMutex.Lock()
		order, ordering := pendingOrders[telegramID]
		orderMutex.Unlock()

		// ✅ If user is in an active order, continue order process
		if ordering {
			handleOrderProcess(bot, update, db, order, availableMeals, userSelectedMeals, pendingOrders, &orderMutex)
			continue
		}

		// ✅ Normal command processing
		switch text {
		case "/start":
			HandleStart(bot, update, db)
		case "/menu", "📜 Menu":
			handleMenu(bot, update, db)
		case "/add_meal":
			handleAddMeal(bot, update, db)
		case "🛒 Order":
			handleOrder(bot, update, db)
		case "✅ Confirm Order":
			handleOrderConfirmation(bot, update, db, pendingOrders, &orderMutex)
		case "❌ Cancel":
			handleOrderCancellation(bot, update)
		default:
			msg := tgbotapi.NewMessage(update.Message.Chat.ID, "I don't understand that command.")
			bot.Send(msg)
		}
	}
}

// HandleStart registers new users and welcomes them
func HandleStart(bot *tgbotapi.BotAPI, update tgbotapi.Update, db *sql.DB) {
	telegramID := update.Message.From.ID

	// Check if user exists
	user, err := users.GetUserByTelegramID(db, int64(telegramID))
	if err != nil {
		log.Printf("Error checking user: %v", err)
		msg := tgbotapi.NewMessage(update.Message.Chat.ID, "⚠️ Error checking user status.")
		bot.Send(msg)
		return
	}

	if user == nil {
		// Request phone number from user
		msg := tgbotapi.NewMessage(update.Message.Chat.ID, "📱 Please send your phone number to register.\nClick the button below:")
		replyMarkup := tgbotapi.NewReplyKeyboard(
			tgbotapi.NewKeyboardButtonRow(
				tgbotapi.NewKeyboardButtonContact("📞 Share Phone Number"),
			),
		)
		msg.ReplyMarkup = replyMarkup
		bot.Send(msg)
		return
	}

	// Welcome message with "Menu" and "Order" buttons
	welcomeText := fmt.Sprintf("👋 Welcome back, %s!\nUse the buttons below to view the menu or place an order.", user.Name)
	msg := tgbotapi.NewMessage(update.Message.Chat.ID, welcomeText)

	// Add the "Menu" and "Order" buttons
	replyMarkup := tgbotapi.NewReplyKeyboard(
		tgbotapi.NewKeyboardButtonRow(
			tgbotapi.NewKeyboardButton("📜 Menu"),
			tgbotapi.NewKeyboardButton("🛒 Order"),
		),
	)
	msg.ReplyMarkup = replyMarkup
	bot.Send(msg)

	// Now send another message with an inline keyboard that has the Mini App link
	inlineMsg := tgbotapi.NewMessage(update.Message.Chat.ID, "🛍️ You can also order through our convenient Mini App:")

	// Create an inline keyboard with a URL button
	miniAppURL := "https://your-mini-app-url.example.com" // Replace with your hosted Mini App URL
	inlineKeyboard := tgbotapi.NewInlineKeyboardMarkup(
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonURL("🛍️ Open Food Store", miniAppURL),
		),
	)

	inlineMsg.ReplyMarkup = inlineKeyboard
	bot.Send(inlineMsg)
}

// handleMenu retrieves available meals from the database
func handleMenu(bot *tgbotapi.BotAPI, update tgbotapi.Update, db *sql.DB) {
	// Fetch meals from database
	meals, err := psDB.GetMeals(db) // Implement this function to retrieve meals
	if err != nil {
		log.Printf("Error fetching meals: %v", err)
		msg := tgbotapi.NewMessage(update.Message.Chat.ID, "⚠️ Failed to load menu.")
		bot.Send(msg)
		return
	}

	// Create meal buttons
	var mealButtons []tgbotapi.KeyboardButton
	for _, meal := range meals {
		mealButtons = append(mealButtons, tgbotapi.NewKeyboardButton(meal.Name))
	}

	// Create keyboard
	keyboard := tgbotapi.NewReplyKeyboard(
		tgbotapi.NewKeyboardButtonRow(mealButtons...),
		tgbotapi.NewKeyboardButtonRow(tgbotapi.NewKeyboardButton("🛒 Order")),
	)

	// Send menu message with buttons
	msg := tgbotapi.NewMessage(update.Message.Chat.ID, "📜 Here is our menu. Select a meal or place an order.")
	msg.ReplyMarkup = keyboard
	bot.Send(msg)
}

func handleAddMeal(bot *tgbotapi.BotAPI, update tgbotapi.Update, db *sql.DB) {
	telegramID := update.Message.From.ID
	text := update.Message.Text

	// ✅ Check if user is an admin
	isAdmin, err := psDB.IsAdmin(db, telegramID)
	if err != nil {
		log.Println("Error checking admin status:", err)
		msg := tgbotapi.NewMessage(update.Message.Chat.ID, "❌ Error checking admin status.")
		bot.Send(msg)
		return
	}
	if !isAdmin {
		msg := tgbotapi.NewMessage(update.Message.Chat.ID, "⚠️ Only admins can add meals!")
		bot.Send(msg)
		return
	}

	mealMutex.Lock()
	defer mealMutex.Unlock()

	// ✅ Check if user is in the meal-adding process
	meal, exists := pendingMeals[telegramID]
	if !exists {
		pendingMeals[telegramID] = &models.Meal{}
		log.Printf("🆕 Starting new meal entry for user: %d\n", telegramID)
		msg := tgbotapi.NewMessage(update.Message.Chat.ID, "🍽️ Enter meal name:")
		bot.Send(msg)
		return
	}

	// ✅ Step 1: Get meal name
	if meal.Name == "" {
		meal.Name = text
		log.Printf("🔹 Meal name received: %s\n", meal.Name)
		msg := tgbotapi.NewMessage(update.Message.Chat.ID, "💰 Enter meal price:")
		bot.Send(msg)
		return
	}

	// ✅ Step 2: Get meal price
	price, err := strconv.Atoi(text)
	if err != nil {
		msg := tgbotapi.NewMessage(update.Message.Chat.ID, "⚠️ Invalid price! Please enter a number.")
		bot.Send(msg)
		return
	}

	meal.Price = price
	log.Printf("💵 Meal price received: %d\n", meal.Price)

	// ✅ Save meal to DB
	err = psDB.AddMeal(db, meal)
	if err != nil {
		log.Println("Error adding meal:", err)
		msg := tgbotapi.NewMessage(update.Message.Chat.ID, "❌ Failed to add meal. Try again.")
		bot.Send(msg)
	} else {
		msg := tgbotapi.NewMessage(update.Message.Chat.ID, fmt.Sprintf("✅ Meal *%s* added successfully!", meal.Name))
		msg.ParseMode = "Markdown"
		bot.Send(msg)
	}

	// ✅ Cleanup
	delete(pendingMeals, telegramID)
	log.Printf("✅ Meal entry completed for user: %d\n", telegramID)
}

// HandleContact saves the user's phone number
func HandleContact(bot *tgbotapi.BotAPI, update tgbotapi.Update, db *sql.DB) {
	telegramID := update.Message.From.ID
	contact := update.Message.Contact

	// Ensure the contact belongs to the sender
	if contact.UserID != telegramID {
		msg := tgbotapi.NewMessage(update.Message.Chat.ID, "⚠️ Please send your own phone number.")
		bot.Send(msg)
		return
	}

	// Register the user in the database
	err := users.RegisterUser(db, models.User{Name: contact.FirstName, PhoneNumber: contact.PhoneNumber, TelegramID: telegramID, Role: "user"})
	if err != nil {
		log.Printf("Error registering user: %v", err)
		msg := tgbotapi.NewMessage(update.Message.Chat.ID, "⚠️ Registration failed. Please try again.")
		bot.Send(msg)
		return
	}

	// Send confirmation and show menu button
	msg := tgbotapi.NewMessage(update.Message.Chat.ID, "✅ Registration successful! You can now place orders.")
	menuButton := tgbotapi.NewReplyKeyboard(
		tgbotapi.NewKeyboardButtonRow(
			tgbotapi.NewKeyboardButton("📜 Menu"),
		),
	)
	msg.ReplyMarkup = menuButton
	bot.Send(msg)
}

func handleMealSelection(bot *tgbotapi.BotAPI, update tgbotapi.Update, db *sql.DB, mealName string) {
	chatID := update.Message.Chat.ID

	// Confirm order
	msg := tgbotapi.NewMessage(chatID, fmt.Sprintf("✅ You selected: %s\nWould you like to confirm your order?", mealName))
	replyMarkup := tgbotapi.NewReplyKeyboard(
		tgbotapi.NewKeyboardButtonRow(
			tgbotapi.NewKeyboardButton("✅ Confirm Order"),
			tgbotapi.NewKeyboardButton("❌ Cancel"),
		),
	)
	msg.ReplyMarkup = replyMarkup

	bot.Send(msg)
}

func handleOrder(bot *tgbotapi.BotAPI, update tgbotapi.Update, db *sql.DB) {
	telegramID := update.Message.From.ID

	// Fetch available meals from the database
	meals, err := psDB.GetMeals(db)
	if err != nil {
		log.Printf("Error fetching meals: %v", err)
		msg := tgbotapi.NewMessage(update.Message.Chat.ID, "⚠️ Error fetching meals.")
		bot.Send(msg)
		return
	}

	// If no meals are available
	if len(meals) == 0 {
		msg := tgbotapi.NewMessage(update.Message.Chat.ID, "🚫 No meals available at the moment.")
		bot.Send(msg)
		return
	}

	// Create meal selection buttons
	var mealButtons []tgbotapi.KeyboardButton
	for _, meal := range meals {
		mealButtons = append(mealButtons, tgbotapi.NewKeyboardButton(meal.Name))
	}

	// Include "Confirm Order" button at the end
	replyMarkup := tgbotapi.NewReplyKeyboard(
		tgbotapi.NewKeyboardButtonRow(mealButtons...),
		tgbotapi.NewKeyboardButtonRow(
			tgbotapi.NewKeyboardButton("✅ Confirm Order"),
			tgbotapi.NewKeyboardButton("❌ Cancel"),
		),
	)

	// Send message with meal options
	msg := tgbotapi.NewMessage(update.Message.Chat.ID, "🍽️ Select a meal to order:")
	msg.ReplyMarkup = replyMarkup
	bot.Send(msg)

	// Save user state for ordering process
	orderMutex.Lock()
	pendingOrders[telegramID] = models.Order{User: models.User{TelegramID: int64(telegramID)}}
	orderMutex.Unlock()
}

func handleOrderProcess(bot *tgbotapi.BotAPI, update tgbotapi.Update, db *sql.DB, order models.Order, availableMeals map[string]models.Meal, userSelectedMeals map[int64]*models.Meal, pendingOrders map[int64]models.Order, orderMutex *sync.Mutex) {
	telegramID := update.Message.From.ID
	telegramID = int64(telegramID)
	text := update.Message.Text

	// Check if input is a valid meal
	if psDB.IsMealName(bot, update, db, text) { // Fetch meal from DB
		if meal, exists := availableMeals[text]; exists { // Check if meal exists
			userSelectedMeals[telegramID] = &meal // Store selected meal

			msg := tgbotapi.NewMessage(update.Message.Chat.ID, "✅ You selected "+meal.Name+". Now, enter the quantity:")
			bot.Send(msg)
			return
		}
	}

	if text == "✅ Confirm Order" {
		// Handle order confirmation logic here
		handleOrderConfirmation(bot, update, db, pendingOrders, orderMutex)
		return
	}

	// Check if input is a number (for quantity)
	quantity, err := strconv.Atoi(text)
	if err != nil {
		msg := tgbotapi.NewMessage(update.Message.Chat.ID, "❌ Invalid input. Select a meal or enter quantity.")
		bot.Send(msg)
		return
	}

	orderMutex.Lock()
	defer orderMutex.Unlock()

	order, exists := pendingOrders[telegramID]
	if !exists {
		msg := tgbotapi.NewMessage(update.Message.Chat.ID, "❌ No pending order found.")
		bot.Send(msg)
		return
	}

	selectedMeal, mealExists := userSelectedMeals[telegramID]
	if !mealExists {
		msg := tgbotapi.NewMessage(update.Message.Chat.ID, "❌ No meal selected. Please select a meal first.")
		bot.Send(msg)
		return
	}

	// Add meal to order
	order.OrderMeal = append(order.OrderMeal, models.OrderMeal{Meal: *selectedMeal, Quantity: quantity})

	// Update global order state
	pendingOrders[telegramID] = order

	msg := tgbotapi.NewMessage(update.Message.Chat.ID, fmt.Sprintf("✅ %d x %s added to your order.", quantity, selectedMeal.Name))
	bot.Send(msg)
}

func handleOrderCancellation(bot *tgbotapi.BotAPI, update tgbotapi.Update) {
	telegramID := update.Message.From.ID

	orderMutex.Lock()
	delete(pendingOrders, telegramID)
	orderMutex.Unlock()

	msg := tgbotapi.NewMessage(update.Message.Chat.ID, "❌ Order cancelled.")
	bot.Send(msg)
}

func handleOrderConfirmation(bot *tgbotapi.BotAPI, update tgbotapi.Update, db *sql.DB, pendingOrders map[int64]models.Order, orderMutex *sync.Mutex) {
	telegramID := update.Message.From.ID

	orderMutex.Lock()
	order, exists := pendingOrders[telegramID]
	if !exists {
		orderMutex.Unlock()
		msg := tgbotapi.NewMessage(update.Message.Chat.ID, "❌ No pending order found.")
		bot.Send(msg)
		return
	}
	delete(pendingOrders, telegramID) // Remove order after confirmation
	orderMutex.Unlock()

	// Fetch user's name and phone number from database
	var fullName, phoneNumber string
	err := db.QueryRow("SELECT full_name, phone_number FROM users WHERE telegram_id = ?", telegramID).Scan(&fullName, &phoneNumber)
	if err != nil {
		fullName = update.Message.From.FirstName // Fallback to Telegram name
		phoneNumber = "Not provided"
	}

	// Calculate total price
	totalPrice := 0
	orderSummary := fmt.Sprintf("🛒 *New Order Received*\n👤 *Customer:* %s\n📞 *Phone:* %s\n\n", fullName, phoneNumber)
	for _, item := range order.OrderMeal {
		itemTotal := item.Quantity * item.Meal.Price
		totalPrice += itemTotal
		orderSummary += fmt.Sprintf("- %d x %s: %d UZS\n", item.Quantity, item.Meal.Name, itemTotal)
	}
	orderSummary += fmt.Sprintf("\n💰 *Total Price:* %d UZS", totalPrice)

	// Send confirmation message to user
	msg := tgbotapi.NewMessage(update.Message.Chat.ID, "✅ Your order has been confirmed! Now, please share your delivery location.")

	// Create a keyboard with the location button
	locationButton := tgbotapi.NewKeyboardButtonLocation("📍 Share Location")
	replyMarkup := tgbotapi.NewReplyKeyboard(
		tgbotapi.NewKeyboardButtonRow(locationButton),
	)
	msg.ReplyMarkup = replyMarkup
	bot.Send(msg)

	// Save order information temporarily until we get the location
	// Can't directly modify a field of a struct in a map
	orderMutex.Lock()
	order.AwaitingLocation = true     // Set the flag before storing
	pendingOrders[telegramID] = order // Store the updated order
	orderMutex.Unlock()
}

func handleLocation(bot *tgbotapi.BotAPI, update tgbotapi.Update, db *sql.DB) {
	telegramID := update.Message.From.ID

	orderMutex.Lock()
	order, exists := pendingOrders[telegramID]
	orderMutex.Unlock()

	if !exists || !order.AwaitingLocation {
		msg := tgbotapi.NewMessage(update.Message.Chat.ID, "⚠️ No pending order found that requires location.")
		bot.Send(msg)
		return
	}

	// Save the location coordinates
	lat := update.Message.Location.Latitude
	lon := update.Message.Location.Longitude
	locationString := fmt.Sprintf("%.6f,%.6f", lat, lon)

	// Update the order with location
	orderMutex.Lock()
	order.Location = locationString   // Update the local copy
	pendingOrders[telegramID] = order // Store it back in the map
	orderMutex.Unlock()

	// Fetch user's name and phone number from database
	var fullName, phoneNumber string
	err := db.QueryRow("SELECT name, phone_number FROM users WHERE telegram_id = $1", telegramID).Scan(&fullName, &phoneNumber)
	if err != nil {
		fullName = update.Message.From.FirstName // Fallback to Telegram name
		phoneNumber = "Not provided"
	}

	// Calculate total price
	totalPrice := 0
	orderSummary := fmt.Sprintf("🛒 *New Order Received*\n👤 *Customer:* %s\n📞 *Phone:* %s\n\n", fullName, phoneNumber)
	for _, item := range order.OrderMeal {
		itemTotal := item.Quantity * item.Meal.Price
		totalPrice += itemTotal
		orderSummary += fmt.Sprintf("- %d x %s: %d UZS\n", item.Quantity, item.Meal.Name, itemTotal)
	}
	orderSummary += fmt.Sprintf("\n💰 *Total Price:* %d UZS", totalPrice)
	orderSummary += fmt.Sprintf("\n📍 *Delivery Location:* [Map](https://maps.google.com/maps?q=%s)", locationString)

	// Send confirmation to user
	msg := tgbotapi.NewMessage(update.Message.Chat.ID, "🚚 Thank you! Your delivery is on the way. We'll update you when your food is ready.")

	// Reset to normal menu
	replyMarkup := tgbotapi.NewReplyKeyboard(
		tgbotapi.NewKeyboardButtonRow(
			tgbotapi.NewKeyboardButton("📜 Menu"),
			tgbotapi.NewKeyboardButton("🛒 Order"),
		),
	)
	msg.ReplyMarkup = replyMarkup
	bot.Send(msg)

	// Send order details to admin group
	adminMsg := tgbotapi.NewMessage(adminGroupID, orderSummary)
	adminMsg.ParseMode = "Markdown"
	bot.Send(adminMsg)

	// Remove from pending orders
	orderMutex.Lock()
	delete(pendingOrders, telegramID)
	orderMutex.Unlock()

	// Save order to database (you'd need to update your DB schema and functions to include location)
	saveOrderWithLocation(db, order)
}

func saveOrderWithLocation(db *sql.DB, order models.Order) {
	// Implementation to save the order with location to the database
	// This would require updating your DB schema to include a location field

	// For now, we'll just use the existing SaveOrder function
	psDB.SaveOrder(db, &order)
}
