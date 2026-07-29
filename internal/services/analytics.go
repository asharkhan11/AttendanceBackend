package services

import (
	"bytes"
	"context"
	"encoding/csv"
	"fmt"
	"io"
	"time"

	fpdf "github.com/go-pdf/fpdf"
	"github.com/google/uuid"
	"github.com/school/attendance-backend/internal/apperror"
	"github.com/school/attendance-backend/internal/repository"
)

type AnalyticsService struct {
	analyticsRepo *repository.AnalyticsRepo
	studentRepo   *repository.StudentRepo
}

func NewAnalyticsService(ar *repository.AnalyticsRepo, sr *repository.StudentRepo) *AnalyticsService {
	return &AnalyticsService{ar, sr}
}

func (s *AnalyticsService) checkClassAccess(ctx context.Context, classID uuid.UUID) error {
	// Authenticated users (admin or teacher) within the school (via RLS) can view class summary
	return nil
}

func parseDateRange(from, to string) (time.Time, time.Time, error) {
	layout := "2006-01-02"
	f, err := time.Parse(layout, from)
	if err != nil {
		return time.Time{}, time.Time{}, apperror.Validation("from must be YYYY-MM-DD")
	}
	t, err := time.Parse(layout, to)
	if err != nil {
		return time.Time{}, time.Time{}, apperror.Validation("to must be YYYY-MM-DD")
	}
	return f, t, nil
}

func (s *AnalyticsService) ClassSummary(ctx context.Context, classID uuid.UUID, from, to string) (any, error) {
	if err := s.checkClassAccess(ctx, classID); err != nil {
		return nil, err
	}
	f, t, err := parseDateRange(from, to)
	if err != nil {
		return nil, err
	}
	summary, err := s.analyticsRepo.ClassSummary(ctx, classID, f, t)
	if err != nil {
		return nil, apperror.Internal(err)
	}
	return summary, nil
}

type StudentSummaryResponse struct {
	StudentID  uuid.UUID                      `json:"student_id"`
	From       string                         `json:"from"`
	To         string                         `json:"to"`
	OverallPct float64                        `json:"overall_pct"`
	BySubject  []*repository.StudentSummaryRow `json:"by_subject"`
	DailyLog   []*repository.StudentDayRow    `json:"daily_log"`
}

func (s *AnalyticsService) StudentSummary(ctx context.Context, studentID uuid.UUID, from, to string) (*StudentSummaryResponse, error) {
	f, t, err := parseDateRange(from, to)
	if err != nil {
		return nil, err
	}
	subjRows, err := s.analyticsRepo.StudentSubjectSummary(ctx, studentID, f, t)
	if err != nil {
		return nil, apperror.Internal(err)
	}
	dayLog, err := s.analyticsRepo.StudentDayLog(ctx, studentID, f, t)
	if err != nil {
		return nil, apperror.Internal(err)
	}
	// Compute overall
	var totalPresent, totalSessions int64
	for _, r := range subjRows {
		totalPresent += r.PresentCount
		totalSessions += r.TotalCount
	}
	var overallPct float64
	if totalSessions > 0 {
		overallPct = float64(totalPresent) / float64(totalSessions) * 100
	}
	return &StudentSummaryResponse{
		StudentID:  studentID,
		From:       from,
		To:         to,
		OverallPct: overallPct,
		BySubject:  subjRows,
		DailyLog:   dayLog,
	}, nil
}

func (s *AnalyticsService) Defaulters(ctx context.Context, classID uuid.UUID, from, to string, threshold float64) (any, error) {
	if err := s.checkClassAccess(ctx, classID); err != nil {
		return nil, err
	}
	f, t, err := parseDateRange(from, to)
	if err != nil {
		return nil, err
	}
	rows, err := s.analyticsRepo.Defaulters(ctx, classID, f, t, threshold)
	if err != nil {
		return nil, apperror.Internal(err)
	}
	return map[string]any{
		"class_id":  classID,
		"from":      from,
		"to":        to,
		"threshold": threshold,
		"defaulters": rows,
	}, nil
}

func (s *AnalyticsService) ClassTrend(ctx context.Context, classID uuid.UUID, from, to, granularity string) (any, error) {
	if err := s.checkClassAccess(ctx, classID); err != nil {
		return nil, err
	}
	f, t, err := parseDateRange(from, to)
	if err != nil {
		return nil, err
	}
	rows, err := s.analyticsRepo.ClassTrend(ctx, classID, f, t, granularity)
	if err != nil {
		return nil, apperror.Internal(err)
	}
	return map[string]any{
		"class_id":    classID,
		"granularity": granularity,
		"trend":       rows,
	}, nil
}

func (s *AnalyticsService) SchoolTrend(ctx context.Context, schoolID uuid.UUID, from, to, granularity string) (any, error) {
	f, t, err := parseDateRange(from, to)
	if err != nil {
		return nil, err
	}
	rows, err := s.analyticsRepo.SchoolTrend(ctx, schoolID, f, t, granularity)
	if err != nil {
		return nil, apperror.Internal(err)
	}
	return map[string]any{
		"school_id":   schoolID,
		"granularity": granularity,
		"trend":       rows,
	}, nil
}

// ExportClassCSV generates a CSV of class attendance roster.
func (s *AnalyticsService) ExportClassCSV(ctx context.Context, classID uuid.UUID, from, to string) ([]byte, error) {
	if err := s.checkClassAccess(ctx, classID); err != nil {
		return nil, err
	}
	f, t, err := parseDateRange(from, to)
	if err != nil {
		return nil, err
	}
	rows, err := s.analyticsRepo.ClassRosterWithAttendance(ctx, classID, f, t)
	if err != nil {
		return nil, apperror.Internal(err)
	}

	var buf bytes.Buffer
	w := csv.NewWriter(&buf)
	_ = w.Write([]string{"student_id", "full_name", "roll_number", "present_count", "total_count", "attendance_pct"})
	for _, r := range rows {
		_ = w.Write([]string{
			r.StudentID.String(),
			r.FullName,
			r.RollNumber,
			fmt.Sprintf("%d", r.PresentCount),
			fmt.Sprintf("%d", r.TotalCount),
			fmt.Sprintf("%.2f", r.AttendancePct),
		})
	}
	w.Flush()
	return buf.Bytes(), nil
}

// ExportClassPDF generates a PDF attendance report for a class.
func (s *AnalyticsService) ExportClassPDF(ctx context.Context, classID uuid.UUID, from, to string) ([]byte, error) {
	if err := s.checkClassAccess(ctx, classID); err != nil {
		return nil, err
	}
	f, t, err := parseDateRange(from, to)
	if err != nil {
		return nil, err
	}
	rows, err := s.analyticsRepo.ClassRosterWithAttendance(ctx, classID, f, t)
	if err != nil {
		return nil, apperror.Internal(err)
	}

	pdf := fpdf.New("P", "mm", "A4", "")
	pdf.AddPage()
	pdf.SetFont("Arial", "B", 16)
	pdf.Cell(0, 10, "Attendance Report")
	pdf.Ln(8)
	pdf.SetFont("Arial", "", 11)
	pdf.Cell(0, 8, fmt.Sprintf("Period: %s to %s", from, to))
	pdf.Ln(10)

	// Table header
	pdf.SetFont("Arial", "B", 10)
	pdf.SetFillColor(230, 230, 230)
	pdf.CellFormat(10, 8, "#", "1", 0, "C", true, 0, "")
	pdf.CellFormat(70, 8, "Name", "1", 0, "L", true, 0, "")
	pdf.CellFormat(25, 8, "Roll No", "1", 0, "C", true, 0, "")
	pdf.CellFormat(25, 8, "Present", "1", 0, "C", true, 0, "")
	pdf.CellFormat(25, 8, "Total", "1", 0, "C", true, 0, "")
	pdf.CellFormat(25, 8, "Pct %", "1", 1, "C", true, 0, "")

	pdf.SetFont("Arial", "", 10)
	for i, r := range rows {
		fill := i%2 == 0
		if fill {
			pdf.SetFillColor(248, 248, 248)
		} else {
			pdf.SetFillColor(255, 255, 255)
		}
		pdf.CellFormat(10, 7, fmt.Sprintf("%d", i+1), "1", 0, "C", fill, 0, "")
		pdf.CellFormat(70, 7, r.FullName, "1", 0, "L", fill, 0, "")
		pdf.CellFormat(25, 7, r.RollNumber, "1", 0, "C", fill, 0, "")
		pdf.CellFormat(25, 7, fmt.Sprintf("%d", r.PresentCount), "1", 0, "C", fill, 0, "")
		pdf.CellFormat(25, 7, fmt.Sprintf("%d", r.TotalCount), "1", 0, "C", fill, 0, "")
		pdf.CellFormat(25, 7, fmt.Sprintf("%.1f%%", r.AttendancePct), "1", 1, "C", fill, 0, "")
	}

	var buf bytes.Buffer
	if err := pdf.Output(io.Writer(&buf)); err != nil {
		return nil, apperror.Internal(err)
	}
	return buf.Bytes(), nil
}

// ExportStudentPDF generates a PDF attendance report for one student.
func (s *AnalyticsService) ExportStudentPDF(ctx context.Context, studentID uuid.UUID, from, to string) ([]byte, error) {
	f, t, err := parseDateRange(from, to)
	if err != nil {
		return nil, err
	}
	subjRows, err := s.analyticsRepo.StudentSubjectSummary(ctx, studentID, f, t)
	if err != nil {
		return nil, apperror.Internal(err)
	}
	dayLog, err := s.analyticsRepo.StudentDayLog(ctx, studentID, f, t)
	if err != nil {
		return nil, apperror.Internal(err)
	}

	pdf := fpdf.New("P", "mm", "A4", "")
	pdf.AddPage()
	pdf.SetFont("Arial", "B", 16)
	pdf.Cell(0, 10, "Student Attendance Report")
	pdf.Ln(8)
	pdf.SetFont("Arial", "", 11)
	pdf.Cell(0, 8, fmt.Sprintf("Student ID: %s", studentID))
	pdf.Ln(6)
	pdf.Cell(0, 8, fmt.Sprintf("Period: %s to %s", from, to))
	pdf.Ln(10)

	// Subject summary
	pdf.SetFont("Arial", "B", 12)
	pdf.Cell(0, 8, "Subject-wise Summary")
	pdf.Ln(8)
	pdf.SetFont("Arial", "B", 10)
	pdf.SetFillColor(230, 230, 230)
	pdf.CellFormat(80, 8, "Subject", "1", 0, "L", true, 0, "")
	pdf.CellFormat(30, 8, "Present", "1", 0, "C", true, 0, "")
	pdf.CellFormat(30, 8, "Total", "1", 0, "C", true, 0, "")
	pdf.CellFormat(30, 8, "Pct %", "1", 1, "C", true, 0, "")
	pdf.SetFont("Arial", "", 10)
	for _, r := range subjRows {
		pdf.CellFormat(80, 7, r.SubjectName, "1", 0, "L", false, 0, "")
		pdf.CellFormat(30, 7, fmt.Sprintf("%d", r.PresentCount), "1", 0, "C", false, 0, "")
		pdf.CellFormat(30, 7, fmt.Sprintf("%d", r.TotalCount), "1", 0, "C", false, 0, "")
		pdf.CellFormat(30, 7, fmt.Sprintf("%.1f%%", r.AttendancePct), "1", 1, "C", false, 0, "")
	}

	// Day log
	pdf.Ln(8)
	pdf.SetFont("Arial", "B", 12)
	pdf.Cell(0, 8, "Day-by-Day Log")
	pdf.Ln(8)
	pdf.SetFont("Arial", "B", 10)
	pdf.SetFillColor(230, 230, 230)
	pdf.CellFormat(35, 8, "Date", "1", 0, "C", true, 0, "")
	pdf.CellFormat(70, 8, "Subject", "1", 0, "L", true, 0, "")
	pdf.CellFormat(20, 8, "Period", "1", 0, "C", true, 0, "")
	pdf.CellFormat(30, 8, "Status", "1", 0, "C", true, 0, "")
	pdf.CellFormat(35, 8, "Remarks", "1", 1, "L", true, 0, "")
	pdf.SetFont("Arial", "", 9)
	for _, r := range dayLog {
		remarks := ""
		if r.Remarks != nil {
			remarks = *r.Remarks
		}
		pdf.CellFormat(35, 6, r.SessionDate.Format("2006-01-02"), "1", 0, "C", false, 0, "")
		pdf.CellFormat(70, 6, r.SubjectName, "1", 0, "L", false, 0, "")
		pdf.CellFormat(20, 6, fmt.Sprintf("%d", r.PeriodNumber), "1", 0, "C", false, 0, "")
		pdf.CellFormat(30, 6, r.Status, "1", 0, "C", false, 0, "")
		pdf.CellFormat(35, 6, remarks, "1", 1, "L", false, 0, "")
	}

	var buf bytes.Buffer
	if err := pdf.Output(io.Writer(&buf)); err != nil {
		return nil, apperror.Internal(err)
	}
	return buf.Bytes(), nil
}

func (s *AnalyticsService) SchoolOverview(ctx context.Context, schoolID uuid.UUID, classID *uuid.UUID, academicYearID *uuid.UUID, from, to string, threshold float64) (*repository.SchoolOverviewData, error) {
	f, t, err := parseDateRange(from, to)
	if err != nil {
		return nil, err
	}
	data, err := s.analyticsRepo.SchoolOverview(ctx, schoolID, classID, academicYearID, f, t, threshold)
	if err != nil {
		return nil, apperror.Internal(err)
	}
	return data, nil
}

func (s *AnalyticsService) SchoolDefaulters(ctx context.Context, schoolID uuid.UUID, classID *uuid.UUID, from, to string, threshold float64) ([]*repository.SchoolDefaulterRow, error) {
	f, t, err := parseDateRange(from, to)
	if err != nil {
		return nil, err
	}
	rows, err := s.analyticsRepo.SchoolDefaulters(ctx, schoolID, classID, f, t, threshold)
	if err != nil {
		return nil, apperror.Internal(err)
	}
	return rows, nil
}

func (s *AnalyticsService) MonthlyMatrix(ctx context.Context, classID uuid.UUID, year, month int) (*repository.MonthlyMatrixData, error) {
	if err := s.checkClassAccess(ctx, classID); err != nil {
		return nil, err
	}
	if year < 2000 || month < 1 || month > 12 {
		return nil, apperror.Validation("invalid year or month")
	}
	data, err := s.analyticsRepo.MonthlyMatrix(ctx, classID, year, month)
	if err != nil {
		return nil, apperror.Internal(err)
	}
	return data, nil
}

func (s *AnalyticsService) ExportMonthlyMatrixCSV(ctx context.Context, classID uuid.UUID, year, month int) ([]byte, error) {
	if err := s.checkClassAccess(ctx, classID); err != nil {
		return nil, err
	}
	data, err := s.analyticsRepo.MonthlyMatrix(ctx, classID, year, month)
	if err != nil {
		return nil, apperror.Internal(err)
	}

	var buf bytes.Buffer
	w := csv.NewWriter(&buf)

	// Header row
	header := []string{"Roll No", "Admission No", "Student Name"}
	for d := 1; d <= data.DaysInMonth; d++ {
		header = append(header, fmt.Sprintf("%02d", d))
	}
	header = append(header, "Present", "Absent", "Late", "Excused")
	_ = w.Write(header)

	for _, st := range data.Students {
		row := []string{st.RollNumber, st.AdmissionNo, st.FullName}
		for d := 1; d <= data.DaysInMonth; d++ {
			dateStr := fmt.Sprintf("%04d-%02d-%02d", year, month, d)
			code := st.DailyStatus[dateStr]
			if code == "" {
				code = "-"
			}
			row = append(row, code)
		}
		row = append(row, fmt.Sprintf("%d", st.TotalPresent), fmt.Sprintf("%d", st.TotalAbsent), fmt.Sprintf("%d", st.TotalLate), fmt.Sprintf("%d", st.TotalExcused))
		_ = w.Write(row)
	}
	w.Flush()
	return buf.Bytes(), nil
}

func (s *AnalyticsService) ExportDefaultersCSV(ctx context.Context, schoolID uuid.UUID, classID *uuid.UUID, from, to string, threshold float64) ([]byte, error) {
	f, t, err := parseDateRange(from, to)
	if err != nil {
		return nil, err
	}
	rows, err := s.analyticsRepo.SchoolDefaulters(ctx, schoolID, classID, f, t, threshold)
	if err != nil {
		return nil, apperror.Internal(err)
	}

	var buf bytes.Buffer
	w := csv.NewWriter(&buf)
	_ = w.Write([]string{"student_id", "full_name", "admission_number", "roll_number", "class_name", "guardian_name", "guardian_phone", "total_sessions", "present_count", "attendance_pct"})
	for _, r := range rows {
		_ = w.Write([]string{
			r.StudentID.String(),
			r.FullName,
			r.AdmissionNo,
			r.RollNumber,
			r.ClassName,
			r.GuardianName,
			r.GuardianPhone,
			fmt.Sprintf("%d", r.TotalSessions),
			fmt.Sprintf("%d", r.PresentCount),
			fmt.Sprintf("%.2f", r.AttendancePct),
		})
	}
	w.Flush()
	return buf.Bytes(), nil
}

func (s *AnalyticsService) AuditLogs(ctx context.Context, schoolID uuid.UUID, from, to string) (any, error) {
	f, t, err := parseDateRange(from, to)
	if err != nil {
		return nil, err
	}
	logs, err := s.analyticsRepo.AuditLogs(ctx, schoolID, f, t)
	if err != nil {
		return nil, apperror.Internal(err)
	}
	return logs, nil
}

func (s *AnalyticsService) ExportAuditLogCSV(ctx context.Context, schoolID uuid.UUID, from, to string) ([]byte, error) {
	f, t, err := parseDateRange(from, to)
	if err != nil {
		return nil, err
	}
	logs, err := s.analyticsRepo.AuditLogs(ctx, schoolID, f, t)
	if err != nil {
		return nil, apperror.Internal(err)
	}

	var buf bytes.Buffer
	w := csv.NewWriter(&buf)
	_ = w.Write([]string{"log_id", "student_name", "roll_number", "class_name", "session_date", "old_status", "new_status", "changed_by", "changed_at"})
	for _, l := range logs {
		oldStat := ""
		if l.OldStatus != nil {
			oldStat = *l.OldStatus
		}
		_ = w.Write([]string{
			l.ID.String(),
			l.StudentName,
			l.RollNumber,
			l.ClassName,
			l.SessionDate.Format("2006-01-02"),
			oldStat,
			l.NewStatus,
			l.ChangedByName,
			l.ChangedAt.Format(time.RFC3339),
		})
	}
	w.Flush()
	return buf.Bytes(), nil
}

func (s *AnalyticsService) GetStudentTemplateCSV() []byte {
	var buf bytes.Buffer
	w := csv.NewWriter(&buf)
	_ = w.Write([]string{"admission_number", "roll_number", "full_name", "gender", "dob", "guardian_name", "guardian_phone"})
	_ = w.Write([]string{"ADM2026099", "01", "Sample Student", "M", "2010-05-15", "Parent Name", "+91-9876543210"})
	_ = w.Write([]string{"ADM2026100", "02", "Jane Doe", "F", "2010-08-20", "John Doe", "+91-9876543211"})
	w.Flush()
	return buf.Bytes()
}

func (s *AnalyticsService) GetTeacherTemplateCSV() []byte {
	var buf bytes.Buffer
	w := csv.NewWriter(&buf)
	_ = w.Write([]string{"full_name", "email", "phone", "password"})
	_ = w.Write([]string{"John Smith", "john.smith@school.edu", "+91-9876543200", "Teacher@1234"})
	_ = w.Write([]string{"Mary Johnson", "mary.johnson@school.edu", "+91-9876543201", "Teacher@1234"})
	w.Flush()
	return buf.Bytes()
}

