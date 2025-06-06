package models

import "time"

// Doctor представляет информацию о враче
type Doctor struct {
	ID             int64
	Name           string
	Specialization string
	Description    string
	PhotoURL       string
	IsActive       bool
	CreatedAt      time.Time
}

// DoctorSchedule представляет расписание врача
type DoctorSchedule struct {
	ID           int64
	DoctorID     int64
	Weekday      int
	StartTime    string
	EndTime      string
	BreakStart   string
	BreakEnd     string
	IsWorkingDay bool
}

// Service представляет стоматологическую услугу
type Service struct {
	ID       int64  `json:"id"`
	Name     string `json:"name"`
	Category string `json:"category"`
	Duration int    `json:"duration"` // в минутах
	Price    int    `json:"price"`
}

// Booking представляет запись на прием
type Booking struct {
	ID             int64
	UserID         int64
	DoctorID       int64
	ServiceID      int64
	Date           string
	Time           string
	Status         string
	PhoneConfirmed bool
	TelegramID     int64
	CreatedAt      time.Time
}

// UserState представляет состояние пользователя в процессе записи
type UserState struct {
	TelegramID int64
	Step       string
	Service    string
	DoctorID   int64
	Date       string
	Time       string
	Phone      string
	CreatedAt  time.Time
}

// AvailableTimeSlot представляет доступный временной слот
type AvailableTimeSlot struct {
	Time       string
	DoctorID   int64
	DoctorName string
}

// IsWeekend проверяет, является ли день выходным
func IsWeekend(date time.Time) bool {
	weekday := date.Weekday()
	return weekday == time.Sunday || weekday == time.Saturday
}

// FormatTime форматирует время в формат HH:MM
func FormatTime(t time.Time) string {
	return t.Format("15:04")
}

// ParseTime парсит время из строки формата HH:MM
func ParseTime(timeStr string) (time.Time, error) {
	return time.Parse("15:04", timeStr)
}
