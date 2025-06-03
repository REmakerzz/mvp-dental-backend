# MVP ChatBot для салона красоты и стоматологии

## Описание

Telegram-бот и веб-админка для записи клиентов в салон красоты и стоматологию.

- Интерактивное меню для клиентов
- Запись на услуги с подтверждением по SMS
- Веб-админка для управления записями и услугами
- Push-уведомления администраторам о новых записях
- Автоотмена неподтвержденных записей
- Экспорт расписания в PDF

## Технологии
- Go 1.21+
- [go-telegram-bot-api](https://github.com/go-telegram-bot-api/telegram-bot-api)
- [gin-gonic/gin](https://github.com/gin-gonic/gin)
- [SQLite](https://github.com/mattn/go-sqlite3)
- [robfig/cron](https://github.com/robfig/cron)
- [gofpdf](https://github.com/jung-kurt/gofpdf)
- [phonenumbers](https://github.com/nyaruka/phonenumbers)

## Быстрый старт

### 1. Клонируйте репозиторий и установите зависимости
```sh
git clone ...
cd MVP_ChatBot
go mod tidy
```

### 2. Настройте переменные окружения
Создайте файл `.env` или экспортируйте переменные:

```
TELEGRAM_BOT_TOKEN=ваш_токен_бота
SMSRU_API_ID=ваш_api_id_от_sms.ru
```

### 3. Запустите миграции (создание БД)
БД инициализируется автоматически при первом запуске.

### 4. Запуск
```sh
go run main.go db.go handlers.go admin.go
```

- Бот работает в Telegram.
- Админка доступна на http://localhost:8080/admin/login

### 5. Дефолтный логин/пароль админки
```
login: admin
password: admin
```

## Переменные окружения
- `TELEGRAM_BOT_TOKEN` — токен Telegram-бота
- `SMSRU_API_ID` — API-ключ SMS-сервиса (если используете sms.ru)

## Структура проекта
- `main.go` — запуск бота и веб-сервера
- `handlers.go` — обработчики Telegram-бота
- `admin.go` — обработчики админки
- `db.go` — работа с БД
- `migrations.sql` — миграции
- `templates/` — HTML-шаблоны админки

## Экспорт расписания
В админке есть кнопка "Экспорт в PDF" — скачивает расписание на выбранный день.

## Автоотмена записей
Неподтвержденные записи автоматически отменяются через 15 минут.

## Интеграция с SMS
В файле `handlers.go` функция `sendSMSCode` — замените заглушку на реальный вызов API вашего SMS-провайдера.

## Docker (опционально)
Можно быстро завернуть в Docker:
```dockerfile
FROM golang:1.21-alpine
WORKDIR /app
COPY . .
RUN go build -o app .
CMD ["./app"]
```

---

**Вопросы и доработки — пишите!** 