package handlers

import (
	"database/sql"
	"fmt"
	"html/template"
	"net/http"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"
)

// Handler представляет обработчик HTTP-запросов
type Handler struct {
	db        *sql.DB
	templates *template.Template
}

// NewHandler создает новый обработчик
func NewHandler(db *sql.DB, templates *template.Template) *Handler {
	return &Handler{
		db:        db,
		templates: templates,
	}
}

// AdminDoctorsHandler обрабатывает страницу управления врачами
func (h *Handler) AdminDoctorsHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method == http.MethodGet {
		// Получаем список врачей
		rows, err := h.db.Query("SELECT id, name, specialization, description, photo_url, is_active FROM doctors ORDER BY name")
		if err != nil {
			http.Error(w, "Ошибка при получении списка врачей", http.StatusInternalServerError)
			return
		}
		defer rows.Close()

		var doctors []Doctor
		for rows.Next() {
			var d Doctor
			if err := rows.Scan(&d.ID, &d.Name, &d.Specialization, &d.Description, &d.PhotoURL, &d.IsActive); err != nil {
				http.Error(w, "Ошибка при сканировании данных врача", http.StatusInternalServerError)
				return
			}
			doctors = append(doctors, d)
		}

		data := struct {
			Doctors []Doctor
		}{
			Doctors: doctors,
		}

		h.templates.ExecuteTemplate(w, "admin_doctors", data)
		return
	}

	if r.Method == http.MethodPost {
		// Добавление нового врача
		name := r.FormValue("name")
		specialization := r.FormValue("specialization")
		description := r.FormValue("description")
		photoURL := r.FormValue("photo_url")

		_, err := h.db.Exec(
			"INSERT INTO doctors (name, specialization, description, photo_url, is_active) VALUES (?, ?, ?, ?, true)",
			name, specialization, description, photoURL,
		)
		if err != nil {
			http.Error(w, "Ошибка при добавлении врача", http.StatusInternalServerError)
			return
		}

		http.Redirect(w, r, "/admin/doctors", http.StatusSeeOther)
		return
	}
}

// AdminDoctorEditHandler обрабатывает страницу редактирования врача
func (h *Handler) AdminDoctorEditHandler(w http.ResponseWriter, r *http.Request) {
	// Получаем ID врача из URL
	parts := strings.Split(r.URL.Path, "/")
	if len(parts) != 4 {
		http.Error(w, "Неверный URL", http.StatusBadRequest)
		return
	}
	doctorID, err := strconv.ParseInt(parts[3], 10, 64)
	if err != nil {
		http.Error(w, "Неверный ID врача", http.StatusBadRequest)
		return
	}

	if r.Method == http.MethodGet {
		// Получаем информацию о враче
		var doctor Doctor
		err := h.db.QueryRow(
			"SELECT id, name, specialization, description, photo_url, is_active FROM doctors WHERE id = ?",
			doctorID,
		).Scan(&doctor.ID, &doctor.Name, &doctor.Specialization, &doctor.Description, &doctor.PhotoURL, &doctor.IsActive)
		if err != nil {
			http.Error(w, "Врач не найден", http.StatusNotFound)
			return
		}

		data := struct {
			Doctor Doctor
		}{
			Doctor: doctor,
		}

		h.templates.ExecuteTemplate(w, "admin_doctor_edit", data)
		return
	}

	if r.Method == http.MethodPost {
		// Обновление информации о враче
		name := r.FormValue("name")
		specialization := r.FormValue("specialization")
		description := r.FormValue("description")
		photoURL := r.FormValue("photo_url")
		isActive := r.FormValue("is_active") == "on"

		_, err := h.db.Exec(
			"UPDATE doctors SET name = ?, specialization = ?, description = ?, photo_url = ?, is_active = ? WHERE id = ?",
			name, specialization, description, photoURL, isActive, doctorID,
		)
		if err != nil {
			http.Error(w, "Ошибка при обновлении врача", http.StatusInternalServerError)
			return
		}

		http.Redirect(w, r, "/admin/doctors", http.StatusSeeOther)
		return
	}
}

// AdminDoctorDeleteHandler обрабатывает удаление врача
func (h *Handler) AdminDoctorDeleteHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Метод не поддерживается", http.StatusMethodNotAllowed)
		return
	}

	// Получаем ID врача из URL
	parts := strings.Split(r.URL.Path, "/")
	if len(parts) != 4 {
		http.Error(w, "Неверный URL", http.StatusBadRequest)
		return
	}
	doctorID, err := strconv.ParseInt(parts[3], 10, 64)
	if err != nil {
		http.Error(w, "Неверный ID врача", http.StatusBadRequest)
		return
	}

	// Удаляем врача
	_, err = h.db.Exec("DELETE FROM doctors WHERE id = ?", doctorID)
	if err != nil {
		http.Error(w, "Ошибка при удалении врача", http.StatusInternalServerError)
		return
	}

	http.Redirect(w, r, "/admin/doctors", http.StatusSeeOther)
}

// AdminDoctorScheduleHandler обрабатывает страницу управления расписанием врача
func (h *Handler) AdminDoctorScheduleHandler(w http.ResponseWriter, r *http.Request) {
	// Получаем ID врача из URL
	parts := strings.Split(r.URL.Path, "/")
	if len(parts) != 4 {
		http.Error(w, "Неверный URL", http.StatusBadRequest)
		return
	}
	doctorID, err := strconv.ParseInt(parts[2], 10, 64)
	if err != nil {
		http.Error(w, "Неверный ID врача", http.StatusBadRequest)
		return
	}

	if r.Method == http.MethodGet {
		// Получаем информацию о враче
		var doctor Doctor
		err := h.db.QueryRow(
			"SELECT id, name, specialization, description, photo_url, is_active FROM doctors WHERE id = ?",
			doctorID,
		).Scan(&doctor.ID, &doctor.Name, &doctor.Specialization, &doctor.Description, &doctor.PhotoURL, &doctor.IsActive)
		if err != nil {
			http.Error(w, "Врач не найден", http.StatusNotFound)
			return
		}

		// Получаем расписание врача
		rows, err := h.db.Query(
			"SELECT id, doctor_id, day_of_week, start_time, end_time, is_working_day FROM doctor_schedules WHERE doctor_id = ? ORDER BY day_of_week, start_time",
			doctorID,
		)
		if err != nil {
			http.Error(w, "Ошибка при получении расписания", http.StatusInternalServerError)
			return
		}
		defer rows.Close()

		var schedules []DoctorSchedule
		for rows.Next() {
			var s DoctorSchedule
			if err := rows.Scan(&s.ID, &s.DoctorID, &s.DayOfWeek, &s.StartTime, &s.EndTime, &s.IsWorkingDay); err != nil {
				http.Error(w, "Ошибка при сканировании данных расписания", http.StatusInternalServerError)
				return
			}
			schedules = append(schedules, s)
		}

		data := struct {
			Doctor    Doctor
			Schedules []DoctorSchedule
		}{
			Doctor:    doctor,
			Schedules: schedules,
		}

		h.templates.ExecuteTemplate(w, "admin_doctor_schedule", data)
		return
	}

	if r.Method == http.MethodPost {
		// Добавление записи в расписание
		dayOfWeek, err := strconv.Atoi(r.FormValue("day_of_week"))
		if err != nil {
			http.Error(w, "Неверный день недели", http.StatusBadRequest)
			return
		}
		startTime := r.FormValue("start_time")
		endTime := r.FormValue("end_time")
		isWorkingDay := r.FormValue("is_working_day") == "on"

		_, err = h.db.Exec(
			"INSERT INTO doctor_schedules (doctor_id, day_of_week, start_time, end_time, is_working_day) VALUES (?, ?, ?, ?, ?)",
			doctorID, dayOfWeek, startTime, endTime, isWorkingDay,
		)
		if err != nil {
			http.Error(w, "Ошибка при добавлении расписания", http.StatusInternalServerError)
			return
		}

		http.Redirect(w, r, fmt.Sprintf("/admin/doctors/%d/schedule", doctorID), http.StatusSeeOther)
		return
	}
}

// AdminDoctorScheduleDeleteHandler обрабатывает удаление записи из расписания
func (h *Handler) AdminDoctorScheduleDeleteHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Метод не поддерживается", http.StatusMethodNotAllowed)
		return
	}

	// Получаем ID врача и ID записи из URL
	parts := strings.Split(r.URL.Path, "/")
	if len(parts) != 6 {
		http.Error(w, "Неверный URL", http.StatusBadRequest)
		return
	}
	doctorID, err := strconv.ParseInt(parts[2], 10, 64)
	if err != nil {
		http.Error(w, "Неверный ID врача", http.StatusBadRequest)
		return
	}
	scheduleID, err := strconv.ParseInt(parts[4], 10, 64)
	if err != nil {
		http.Error(w, "Неверный ID записи расписания", http.StatusBadRequest)
		return
	}

	// Удаляем запись из расписания
	_, err = h.db.Exec("DELETE FROM doctor_schedules WHERE id = ? AND doctor_id = ?", scheduleID, doctorID)
	if err != nil {
		http.Error(w, "Ошибка при удалении записи расписания", http.StatusInternalServerError)
		return
	}

	http.Redirect(w, r, fmt.Sprintf("/admin/doctors/%d/schedule", doctorID), http.StatusSeeOther)
}

// adminBookingsHandler обрабатывает страницу записей
func adminBookingsHandler(db *sql.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		// Получаем список записей
		rows, err := db.Query(`
			SELECT b.id, b.date, b.time, b.status, 
				   s.name as service_name, 
				   u.name as client_name, u.phone
			FROM bookings b
			JOIN services s ON b.service_id = s.id
			JOIN users u ON b.user_id = u.id
			ORDER BY b.date DESC, b.time DESC
		`)
		if err != nil {
			c.HTML(http.StatusInternalServerError, "error.html", gin.H{
				"error": "Ошибка при получении списка записей",
			})
			return
		}
		defer rows.Close()

		var bookings []struct {
			ID          int64
			Date        string
			Time        string
			Status      string
			ServiceName string
			ClientName  string
			Phone       string
		}

		for rows.Next() {
			var b struct {
				ID          int64
				Date        string
				Time        string
				Status      string
				ServiceName string
				ClientName  string
				Phone       string
			}
			if err := rows.Scan(&b.ID, &b.Date, &b.Time, &b.Status, &b.ServiceName, &b.ClientName, &b.Phone); err != nil {
				c.HTML(http.StatusInternalServerError, "error.html", gin.H{
					"error": "Ошибка при сканировании данных записи",
				})
				return
			}
			bookings = append(bookings, b)
		}

		c.HTML(http.StatusOK, "admin_bookings.html", gin.H{
			"bookings": bookings,
		})
	}
}
