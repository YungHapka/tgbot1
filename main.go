package main

import (
	"bufio"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/PuerkitoBio/goquery"
	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
	"github.com/joho/godotenv"
)

const userFile = "users.txt"

var userIDs []int64

// Функция загрузки пользователей
func loadUsers() {
	file, err := os.Open(userFile)
	if err != nil {
		if os.IsNotExist(err) {
			log.Println("Файл с пользователями не найден, создаем новый.")
		} else {
			log.Println("Ошибка при открытии файла пользователей:", err)
		}
		return
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		var userID int64
		fmt.Sscanf(scanner.Text(), "%d", &userID)
		userIDs = append(userIDs, userID)
	}
	log.Println("Загружены пользователи:", userIDs)
}

// Функция сохранения нового пользователя
func saveUser(userID int64) {
	for _, id := range userIDs {
		if id == userID {
			return
		}
	}
	userIDs = append(userIDs, userID)

	file, err := os.OpenFile(userFile, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		log.Println("Ошибка при открытии файла пользователей:", err)
		return
	}
	defer file.Close()

	writer := bufio.NewWriter(file)
	fmt.Fprintln(writer, userID)
	writer.Flush()
	log.Println("Добавлен новый пользователь:", userID)
}

// Функция получения расписания Джона Уика
func getJohnWickSchedule() string {
	url := "https://tv.yandex.ru/search?text=Джон+Уик"
	res, err := http.Get(url)
	if err != nil {
		log.Println("Ошибка запроса:", err)
		return "Ошибка при получении данных."
	}
	defer res.Body.Close()

	log.Printf("Получен ответ от %s, статус: %d", url, res.StatusCode)

	if res.StatusCode != 200 {
		return fmt.Sprintf("Ошибка: не удалось получить данные. Статус: %d", res.StatusCode)
	}

	body, err := io.ReadAll(res.Body)
	if err != nil {
		log.Println("Ошибка при чтении тела ответа:", err)
		return "Ошибка при обработке ответа."
	}

	log.Println("Ответ сервера:", string(body))

	doc, err := goquery.NewDocumentFromReader(strings.NewReader(string(body)))
	if err != nil {
		log.Println("Ошибка обработки HTML:", err)
		return "Ошибка при анализе данных."
	}

	var scheduleInfo string
	doc.Find(".serp-item").Each(func(i int, s *goquery.Selection) {
		if strings.Contains(s.Text(), "Джон Уик") {
			time := s.Find(".serp-item__time").Text()
			channel := s.Find(".serp-item__channel").Text()
			scheduleInfo += fmt.Sprintf("Фильм: Джон Уик\nВремя: %s\nКанал: %s\n\n", time, channel)
		}
	})

	if scheduleInfo == "" {
		return "Сегодня нет Джонов Уиков"
	}
	return scheduleInfo
}

// Функция отправки сообщения всем пользователям
func sendTelegramMessage(bot *tgbotapi.BotAPI, message string) {
	for _, userID := range userIDs {
		msg := tgbotapi.NewMessage(userID, message)
		_, err := bot.Send(msg)
		if err != nil {
			log.Println("Ошибка отправки сообщения:", err)
		}
	}
	log.Println("Сообщение отправлено всем пользователям!")
}

// Функция обработки команды /start
func handleStart(bot *tgbotapi.BotAPI, update tgbotapi.Update) {
	userID := update.Message.Chat.ID
	saveUser(userID)

	keyboard := tgbotapi.NewReplyKeyboard(
		tgbotapi.NewKeyboardButtonRow(
			tgbotapi.NewKeyboardButton("Получить расписание"),
			tgbotapi.NewKeyboardButton("Подписаться на обновления"),
			tgbotapi.NewKeyboardButton("Отписаться от обновлений"),
		),
	)

	msg := tgbotapi.NewMessage(userID, "Привет! Я твой бот для расписания Джона Уика. Выбери действие:")
	msg.ReplyMarkup = keyboard
	_, err := bot.Send(msg)
	if err != nil {
		log.Println("Ошибка отправки сообщения:", err)
	}
}

// Функция обработки нажатий кнопок
func handleCallback(bot *tgbotapi.BotAPI, update tgbotapi.Update) {
	if update.CallbackQuery != nil {
		data := update.CallbackQuery.Data
		userID := update.CallbackQuery.Message.Chat.ID

		var response string
		switch data {
		case "get_schedule":
			response = getJohnWickSchedule()
		case "subscribe":
			saveUser(userID)
			response = "Ты теперь подписан на обновления!"
		case "unsubscribe":
			response = "Ты отписан от обновлений!"
		}

		msg := tgbotapi.NewMessage(userID, response)
		bot.Send(msg)
	}
}

// Главная функция
func main() {
	err := godotenv.Load()
	if err != nil {
		log.Println("Файл .env не найден, используем переменные окружения")
	}

	botToken := os.Getenv("BOT_TOKEN")
	if botToken == "" {
		log.Fatal("BOT_TOKEN не задан")
	}

	bot, err := tgbotapi.NewBotAPI(botToken)
	if err != nil {
		log.Fatal("Ошибка при авторизации бота:", err)
	}

	log.Printf("Авторизован на аккаунте %s", bot.Self.UserName)

	loadUsers()

	u := tgbotapi.UpdateConfig{
		Offset:  0,
		Timeout: 60,
	}

	updates := bot.GetUpdatesChan(u)

	go func() {
		for update := range updates {
			if update.Message != nil {
				log.Printf("Получено сообщение от %d: %s", update.Message.Chat.ID, update.Message.Text)
				if update.Message.Text == "/start" {
					handleStart(bot, update)
				} else {
					msg := tgbotapi.NewMessage(update.Message.Chat.ID, "Спасибо за сообщение!")
					bot.Send(msg)
				}
			}

			if update.CallbackQuery != nil {
				handleCallback(bot, update)
			}
		}
	}()

	for {
		now := time.Now()
		log.Printf("Текущее время: %s", now.Format("15:04"))
		if now.Hour() == 10 && now.Minute() == 0 {
			schedule := getJohnWickSchedule()
			sendTelegramMessage(bot, schedule)
			log.Println("Данные отправлены, ждем следующего дня...")
			time.Sleep(61 * time.Second)
		}
		time.Sleep(30 * time.Second)
	}
}
