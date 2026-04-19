package bot

import "github.com/go-telegram/bot/models"

// kbRow builds a keyboard row with inline buttons in the order they appear.
func kbRow(btns ...models.InlineKeyboardButton) []models.InlineKeyboardButton {
	return btns
}

// btn creates a callback-data inline button.
func btn(text, data string) models.InlineKeyboardButton {
	return models.InlineKeyboardButton{Text: text, CallbackData: data}
}

// urlBtn creates a URL inline button.
func urlBtn(text, url string) models.InlineKeyboardButton {
	return models.InlineKeyboardButton{Text: text, URL: url}
}

// markup composes a set of rows into a ready-to-send InlineKeyboardMarkup.
func markup(rows ...[]models.InlineKeyboardButton) *models.InlineKeyboardMarkup {
	return &models.InlineKeyboardMarkup{InlineKeyboard: rows}
}

// backRow renders a single "⬅ Back" row that returns to the main menu.
func backRow(label string) []models.InlineKeyboardButton {
	return kbRow(btn(label, CBMainMenu))
}
