package server

import (
	"net/http"

	"github.com/go-chi/chi/v5"
	chimw "github.com/go-chi/chi/v5/middleware"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/school/attendance-backend/internal/auth"
	"github.com/school/attendance-backend/internal/config"
	"github.com/school/attendance-backend/internal/handlers"
	mw "github.com/school/attendance-backend/internal/middleware"
	"github.com/school/attendance-backend/internal/models"
	"github.com/school/attendance-backend/internal/repository"
	"github.com/school/attendance-backend/internal/services"

	_ "github.com/school/attendance-backend/docs"
	httpSwagger "github.com/swaggo/http-swagger/v2"
)

func NewRouter(cfg *config.Config, pool *pgxpool.Pool) http.Handler {
	r := chi.NewRouter()

	// ─── Build service dependencies ───────────────────────────────────────────
	db := repository.NewDB(pool)

	schoolRepo := repository.NewSchoolRepo(pool)
	userRepo := repository.NewUserRepo(db)
	classRepo := repository.NewClassRepo(db)
	yearRepo := repository.NewAcademicYearRepo(db)
	studentRepo := repository.NewStudentRepo(db)
	attendanceRepo := repository.NewAttendanceRepo(db)
	analyticsRepo := repository.NewAnalyticsRepo(db)
	calendarRepo := repository.NewCalendarRepo(db)
	whatsappRepo := repository.NewWhatsAppRepo(db)

	jwtSvc := auth.NewJWTService(cfg.JWTSecret, cfg.JWTAccessTTL)
	passSvc := auth.NewPasswordService(cfg.Argon2MemoryKB, cfg.Argon2Iterations, cfg.Argon2Parallelism)

	studentBulkRepo := repository.NewStudentBulkRepository(db)
	attendanceBulkRepo := repository.NewAttendanceBulkRepository(db)

	authSvc := services.NewAuthService(schoolRepo, userRepo, jwtSvc, passSvc, cfg.JWTAccessTTL, cfg.JWTRefreshTTL)
	classSvc := services.NewClassService(classRepo, yearRepo)
	teacherSvc := services.NewTeacherService(userRepo, passSvc)
	studentSvc := services.NewStudentService(studentRepo, studentBulkRepo)
	yearSvc := services.NewAcademicYearService(yearRepo)
	attendanceSvc := services.NewAttendanceService(attendanceRepo, attendanceBulkRepo, studentRepo, classRepo)
	whatsappClient := services.NewWhatsAppClient(cfg)
	attendanceSvc.SetWhatsApp(whatsappRepo, whatsappClient)
	analyticsSvc := services.NewAnalyticsService(analyticsRepo, studentRepo)
	calendarSvc := services.NewCalendarService(calendarRepo)

	// ─── Build handlers ───────────────────────────────────────────────────────
	authH := handlers.NewAuthHandler(authSvc)
	classH := handlers.NewClassHandler(classSvc, studentSvc)
	teacherH := handlers.NewTeacherHandler(teacherSvc)
	studentH := handlers.NewStudentHandler(studentSvc)
	yearH := handlers.NewAcademicYearHandler(yearSvc)
	attendanceH := handlers.NewAttendanceHandler(attendanceSvc)
	analyticsH := handlers.NewAnalyticsHandler(analyticsSvc)
	calendarH := handlers.NewCalendarHandler(calendarSvc)
	webhookH := handlers.NewWhatsAppWebhookHandler(cfg, whatsappRepo)

	// ─── Global middleware ────────────────────────────────────────────────────
	r.Use(chimw.RealIP)
	r.Use(mw.RequestID)
	r.Use(mw.Logger)
	r.Use(mw.Recoverer)
	r.Use(mw.CORS(cfg.CORSAllowedOrigins))

	// ─── Swagger ──────────────────────────────────────────────────────────────
	r.Get("/swagger/*", httpSwagger.Handler(
		httpSwagger.URL("/swagger/doc.json"),
	))

	// ─── Health / readiness ───────────────────────────────────────────────────
	r.Get("/healthz", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"status":"ok"}`))
	})
	r.Get("/readyz", func(w http.ResponseWriter, r *http.Request) {
		if err := db.Ping(r.Context()); err != nil {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusServiceUnavailable)
			_, _ = w.Write([]byte(`{"status":"db_unreachable"}`))
			return
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"status":"ok"}`))
	})

	// ─── Meta WhatsApp Webhook endpoints (unauthenticated) ─────────────────────
	r.Get("/webhook/whatsapp", webhookH.VerifyWebhook)
	r.Post("/webhook/whatsapp", webhookH.HandleWebhook)
	r.Get("/api/v1/webhooks/whatsapp", webhookH.VerifyWebhook)
	r.Post("/api/v1/webhooks/whatsapp", webhookH.HandleWebhook)

	// ─── Auth routes (rate limited) ───────────────────────────────────────────
	r.Group(func(r chi.Router) {
		r.Use(mw.RateLimit(cfg.RateLimitLoginPerMin))
		r.Post("/api/v1/auth/login", authH.Login)
		r.Post("/api/v1/auth/refresh", authH.Refresh)
	})

	// ─── Authenticated routes ─────────────────────────────────────────────────
	r.Group(func(r chi.Router) {
		r.Use(mw.JWTAuth(jwtSvc))
		r.Use(mw.TenantContext(db))

		// logout (needs JWT but no tenant tx for the revoke)
		r.Post("/api/v1/auth/logout", authH.Logout)

		// ── Me ────────────────────────────────────────────────────────────────
		r.Get("/api/v1/me", authH.Me)

		// ── Admin-only routes ─────────────────────────────────────────────────
		r.Group(func(r chi.Router) {
			r.Use(mw.RequireRole(models.RoleAdmin))

			// Academic Years (Sessions)
			r.Get("/api/v1/academic-years", yearH.List)
			r.Post("/api/v1/academic-years", yearH.Create)
			r.Patch("/api/v1/academic-years/{id}/set-current", yearH.SetCurrent)
			r.Patch("/api/v1/academic-years/{id}/archive", yearH.Archive)
			r.Patch("/api/v1/academic-years/{id}/unarchive", yearH.Unarchive)
			r.Post("/api/v1/academic-years/promote-students", yearH.PromoteStudents)
			r.Post("/api/v1/academic-years/copy-classes", yearH.CopyClasses)
			r.Post("/api/v1/academic-years/copy-teachers", yearH.CopyTeachers)

			// Classes (admin only write operations)
			r.Get("/api/v1/classes", classH.List)
			r.Post("/api/v1/classes", classH.Create)
			r.Put("/api/v1/classes/{id}/teacher", classH.AssignTeacher)
			r.Delete("/api/v1/classes/{id}", classH.Delete)

			// Teachers
			r.Get("/api/v1/teachers", teacherH.List)
			r.Post("/api/v1/teachers", teacherH.Create)
			r.Put("/api/v1/teachers/{id}", teacherH.Update)
			r.Patch("/api/v1/teachers/{id}/deactivate", teacherH.Deactivate)
			r.Patch("/api/v1/teachers/{id}/activate", teacherH.Activate)
			r.Delete("/api/v1/teachers/{id}", teacherH.Delete)
			r.Post("/api/v1/teachers/{id}/reset-password", teacherH.ResetPassword)

			// Students (admin full CRUD)
			r.Get("/api/v1/students", studentH.List)
			r.Get("/api/v1/students/next-info", studentH.NextStudentInfo)
			r.Post("/api/v1/students", studentH.Create)
			r.Post("/api/v1/students/bulk-import", studentH.BulkImport)
			r.Put("/api/v1/students/{id}", studentH.Update)
			r.Patch("/api/v1/students/{id}/deactivate", studentH.Deactivate)
			r.Delete("/api/v1/students/{id}", studentH.Delete)

			// Reports (admin only)
			r.Get("/api/v1/reports/class/{class_id}/export", analyticsH.ExportClass)
			r.Get("/api/v1/reports/student/{student_id}/export", analyticsH.ExportStudent)
			r.Get("/api/v1/reports/defaulters/export", analyticsH.ExportDefaulters)
			r.Get("/api/v1/reports/audit-log", analyticsH.AuditLogs)
			r.Get("/api/v1/reports/audit-log/export", analyticsH.ExportAuditLogs)

			// Templates & Imports
			r.Get("/api/v1/templates/students.csv", analyticsH.StudentTemplate)
			r.Get("/api/v1/templates/teachers.csv", analyticsH.TeacherTemplate)

			// School Calendar / Days (admin write)
			r.Put("/api/v1/school-days/weekly", calendarH.UpdateWeeklySchedule)
			r.Post("/api/v1/school-days/holidays", calendarH.CreateDateHoliday)
			r.Delete("/api/v1/school-days/holidays/{id}", calendarH.DeleteDateHoliday)

		})

		// ── Shared routes (admin + teacher) ───────────────────────────────────
		r.Group(func(r chi.Router) {
			r.Use(mw.RequireRole(models.RoleAdmin, models.RoleTeacher))

			// School Calendar / Days (read access)
			r.Get("/api/v1/school-days/weekly", calendarH.GetWeeklySchedule)
			r.Get("/api/v1/school-days/holidays", calendarH.GetDateHolidays)

			// User Profile & Security
			r.Put("/api/v1/users/me", authH.UpdateProfile)
			r.Post("/api/v1/users/me/change-password", authH.ChangePassword)

			// Teacher class assignment
			r.Get("/api/v1/classes/my-class", classH.GetMyClass)

			// Students: teachers can view roster of their classes
			r.Get("/api/v1/classes/{id}/students", classH.ListStudents)

			// Attendance sessions
			r.Post("/api/v1/attendance/sessions", attendanceH.CreateSession)
			r.Get("/api/v1/attendance/sessions", attendanceH.ListSessions)
			r.Get("/api/v1/attendance/sessions/{id}", attendanceH.GetSession)
			r.Post("/api/v1/attendance/sessions/{id}/mark", attendanceH.BulkMark)
			r.Patch("/api/v1/attendance/sessions/{id}/mark/{student_id}", attendanceH.SingleMark)
			r.Post("/api/v1/attendance/sessions/{id}/close", attendanceH.CloseSession)
			r.Post("/api/v1/attendance/sessions/{id}/reopen", attendanceH.ReopenSession)

			// Analytics & Monthly Reports (service enforces class-teacher scoping for teacher role)
			r.Get("/api/v1/analytics/school/overview", analyticsH.SchoolOverview)
			r.Get("/api/v1/analytics/school/defaulters", analyticsH.SchoolDefaulters)
			r.Get("/api/v1/analytics/school/trend", analyticsH.SchoolTrend)
			r.Get("/api/v1/analytics/class/{class_id}/summary", analyticsH.ClassSummary)
			r.Get("/api/v1/analytics/class/{class_id}/defaulters", analyticsH.Defaulters)
			r.Get("/api/v1/analytics/class/{class_id}/trend", analyticsH.ClassTrend)
			r.Get("/api/v1/analytics/student/{student_id}/summary", analyticsH.StudentSummary)
			r.Get("/api/v1/reports/monthly-matrix/{class_id}", analyticsH.MonthlyMatrix)
			r.Get("/api/v1/reports/monthly-matrix/{class_id}/export", analyticsH.ExportMonthlyMatrix)
		})
	})

	return r
}
