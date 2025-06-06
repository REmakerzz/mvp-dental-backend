package handlers

import (
	"database/sql"
	"net/http"

	"github.com/gin-gonic/gin"
)

func AdminDoctorsHandler(db *sql.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		if c.Request.Method == "GET" {
			rows, err := db.Query(`
				SELECT id, name, specialization, experience, education
				FROM doctors
				ORDER BY name
			`)
			if err != nil {
				c.HTML(http.StatusInternalServerError, "error.html", gin.H{
					"error": "Ошибка при получении данных",
				})
				return
			}
			defer rows.Close()

			var doctors []struct {
				ID             int64
				Name           string
				Specialization string
				Experience     int
				Education      string
			}

			for rows.Next() {
				var d struct {
					ID             int64
					Name           string
					Specialization string
					Experience     int
					Education      string
				}
				if err := rows.Scan(&d.ID, &d.Name, &d.Specialization, &d.Experience, &d.Education); err == nil {
					doctors = append(doctors, d)
				}
			}

			c.HTML(http.StatusOK, "doctors.html", gin.H{
				"doctors": doctors,
			})
			return
		}

		// POST запрос - добавление нового врача
		name := c.PostForm("name")
		specialization := c.PostForm("specialization")
		experience := c.PostForm("experience")
		education := c.PostForm("education")

		if name == "" || specialization == "" || experience == "" || education == "" {
			c.HTML(http.StatusBadRequest, "doctors.html", gin.H{
				"error": "Все поля должны быть заполнены",
			})
			return
		}

		_, err := db.Exec(`
			INSERT INTO doctors (name, specialization, experience, education)
			VALUES (?, ?, ?, ?)
		`, name, specialization, experience, education)

		if err != nil {
			c.HTML(http.StatusInternalServerError, "doctors.html", gin.H{
				"error": "Ошибка при добавлении врача",
			})
			return
		}

		c.Redirect(http.StatusFound, "/admin/doctors")
	}
}

func AdminEditDoctorHandler(db *sql.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		doctorID := c.Param("doctor_id")
		if doctorID == "" {
			c.JSON(http.StatusBadRequest, gin.H{"error": "ID врача не указан"})
			return
		}

		if c.Request.Method == "GET" {
			var doctor struct {
				ID             int64
				Name           string
				Specialization string
				Experience     int
				Education      string
			}

			err := db.QueryRow(`
				SELECT id, name, specialization, experience, education
				FROM doctors
				WHERE id = ?
			`, doctorID).Scan(&doctor.ID, &doctor.Name, &doctor.Specialization, &doctor.Experience, &doctor.Education)

			if err != nil {
				c.HTML(http.StatusInternalServerError, "error.html", gin.H{
					"error": "Ошибка при получении данных",
				})
				return
			}

			c.HTML(http.StatusOK, "edit_doctor.html", gin.H{
				"doctor": doctor,
			})
			return
		}

		// POST запрос - обновление врача
		name := c.PostForm("name")
		specialization := c.PostForm("specialization")
		experience := c.PostForm("experience")
		education := c.PostForm("education")

		if name == "" || specialization == "" || experience == "" || education == "" {
			c.HTML(http.StatusBadRequest, "edit_doctor.html", gin.H{
				"error": "Все поля должны быть заполнены",
			})
			return
		}

		_, err := db.Exec(`
			UPDATE doctors
			SET name = ?, specialization = ?, experience = ?, education = ?
			WHERE id = ?
		`, name, specialization, experience, education, doctorID)

		if err != nil {
			c.HTML(http.StatusInternalServerError, "edit_doctor.html", gin.H{
				"error": "Ошибка при обновлении данных врача",
			})
			return
		}

		c.Redirect(http.StatusFound, "/admin/doctors")
	}
}

func AdminDeleteDoctorHandler(db *sql.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		doctorID := c.Param("doctor_id")
		if doctorID == "" {
			c.JSON(http.StatusBadRequest, gin.H{"error": "ID врача не указан"})
			return
		}

		_, err := db.Exec("DELETE FROM doctors WHERE id = ?", doctorID)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Ошибка при удалении врача"})
			return
		}

		c.Redirect(http.StatusFound, "/admin/doctors")
	}
}

func AdminDoctorScheduleHandler(db *sql.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		doctorID := c.Param("doctor_id")
		if doctorID == "" {
			c.JSON(http.StatusBadRequest, gin.H{"error": "ID врача не указан"})
			return
		}

		if c.Request.Method == "GET" {
			rows, err := db.Query(`
				SELECT id, day_of_week, start_time, end_time
				FROM doctor_schedule
				WHERE doctor_id = ?
				ORDER BY day_of_week, start_time
			`, doctorID)
			if err != nil {
				c.HTML(http.StatusInternalServerError, "error.html", gin.H{
					"error": "Ошибка при получении данных",
				})
				return
			}
			defer rows.Close()

			var schedules []struct {
				ID        int64
				DayOfWeek int
				StartTime string
				EndTime   string
			}

			for rows.Next() {
				var s struct {
					ID        int64
					DayOfWeek int
					StartTime string
					EndTime   string
				}
				if err := rows.Scan(&s.ID, &s.DayOfWeek, &s.StartTime, &s.EndTime); err == nil {
					schedules = append(schedules, s)
				}
			}

			c.HTML(http.StatusOK, "doctor_schedule.html", gin.H{
				"doctor_id": doctorID,
				"schedules": schedules,
			})
			return
		}

		// POST запрос - добавление расписания
		dayOfWeek := c.PostForm("day_of_week")
		startTime := c.PostForm("start_time")
		endTime := c.PostForm("end_time")

		if dayOfWeek == "" || startTime == "" || endTime == "" {
			c.HTML(http.StatusBadRequest, "doctor_schedule.html", gin.H{
				"error": "Все поля должны быть заполнены",
			})
			return
		}

		_, err := db.Exec(`
			INSERT INTO doctor_schedule (doctor_id, day_of_week, start_time, end_time)
			VALUES (?, ?, ?, ?)
		`, doctorID, dayOfWeek, startTime, endTime)

		if err != nil {
			c.HTML(http.StatusInternalServerError, "doctor_schedule.html", gin.H{
				"error": "Ошибка при добавлении расписания",
			})
			return
		}

		c.Redirect(http.StatusFound, "/admin/doctors/"+doctorID+"/schedule")
	}
}

func AdminDeleteScheduleHandler(db *sql.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		doctorID := c.Param("doctor_id")
		scheduleID := c.Param("schedule_id")
		if doctorID == "" || scheduleID == "" {
			c.JSON(http.StatusBadRequest, gin.H{"error": "ID врача или расписания не указан"})
			return
		}

		_, err := db.Exec("DELETE FROM doctor_schedule WHERE id = ? AND doctor_id = ?", scheduleID, doctorID)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Ошибка при удалении расписания"})
			return
		}

		c.Redirect(http.StatusFound, "/admin/doctors/"+doctorID+"/schedule")
	}
}
