-- scripts/seed.sql
-- Seeds demo school with rich, production-grade sample data for local development.
-- Password for Admin: Admin@1234
-- Password for Teachers: Teacher@1234

DO $$
DECLARE
  v_school_id UUID := 'a0000000-0000-0000-0000-000000000001';
BEGIN
  DELETE FROM attendance_audit_log  WHERE school_id = v_school_id;
  DELETE FROM attendance_records    WHERE school_id = v_school_id;
  DELETE FROM attendance_sessions   WHERE school_id = v_school_id;
  DELETE FROM students              WHERE school_id = v_school_id;
  DELETE FROM classes               WHERE school_id = v_school_id;
  DELETE FROM academic_years        WHERE school_id = v_school_id;
  DELETE FROM refresh_tokens        WHERE user_id IN (SELECT id FROM users WHERE school_id = v_school_id);
  DELETE FROM users                 WHERE school_id = v_school_id;
  DELETE FROM schools               WHERE id = v_school_id;
END $$;

-- School
INSERT INTO schools (id, name, code, is_active)
VALUES ('a0000000-0000-0000-0000-000000000001', 'Demo High School', 'demo-school', true);

-- Academic Years
INSERT INTO academic_years (id, school_id, name, start_date, end_date, is_current, is_archived) VALUES
  ('a0000000-0000-0000-0000-000000000009', 'a0000000-0000-0000-0000-000000000001', '2025-2026', '2025-06-01', '2026-03-31', false, true),
  ('a0000000-0000-0000-0000-000000000010', 'a0000000-0000-0000-0000-000000000001', '2026-2027', '2026-06-01', '2027-03-31', true, false);

-- Admin
INSERT INTO users (id, school_id, role, full_name, email, password_hash, is_active) VALUES
  ('a0000000-0000-0000-0000-000000000020', 'a0000000-0000-0000-0000-000000000001', 'admin', 'School Admin', 'admin@demo-school.edu',
   '$argon2id$v=19$m=65536,t=3,p=2$c29tZXNhbHRzb21lc2FsdA$GZOZnVsuIl859JTA3EJ6HQbji+o9QcsazcWmDssypW4', true);

-- Teachers
INSERT INTO users (id, school_id, role, full_name, email, phone, password_hash, is_active) VALUES
  ('a0000000-0000-0000-0000-000000000021', 'a0000000-0000-0000-0000-000000000001', 'teacher', 'Priya Sharma', 'priya@demo-school.edu', '+91-9876543210', '$argon2id$v=19$m=65536,t=3,p=2$c29tZXNhbHRzb21lc2FsdA$GZOZnVsuIl859JTA3EJ6HQbji+o9QcsazcWmDssypW4', true),
  ('a0000000-0000-0000-0000-000000000022', 'a0000000-0000-0000-0000-000000000001', 'teacher', 'Rahul Mehta', 'rahul@demo-school.edu', '+91-9876543211', '$argon2id$v=19$m=65536,t=3,p=2$c29tZXNhbHRzb21lc2FsdA$GZOZnVsuIl859JTA3EJ6HQbji+o9QcsazcWmDssypW4', true),
  ('a0000000-0000-0000-0000-000000000023', 'a0000000-0000-0000-0000-000000000001', 'teacher', 'Amit Verma', 'amit@demo-school.edu', '+91-9876543212', '$argon2id$v=19$m=65536,t=3,p=2$c29tZXNhbHRzb21lc2FsdA$GZOZnVsuIl859JTA3EJ6HQbji+o9QcsazcWmDssypW4', true),
  ('a0000000-0000-0000-0000-000000000024', 'a0000000-0000-0000-0000-000000000001', 'teacher', 'Sunita Rao', 'sunita@demo-school.edu', '+91-9876543213', '$argon2id$v=19$m=65536,t=3,p=2$c29tZXNhbHRzb21lc2FsdA$GZOZnVsuIl859JTA3EJ6HQbji+o9QcsazcWmDssypW4', true);

-- Classes
INSERT INTO classes (id, school_id, academic_year_id, grade_level, section, class_teacher_id) VALUES
  ('a0000000-0000-0000-0000-000000000040', 'a0000000-0000-0000-0000-000000000001', 'a0000000-0000-0000-0000-000000000010', 9, 'A', 'a0000000-0000-0000-0000-000000000021'),
  ('a0000000-0000-0000-0000-000000000041', 'a0000000-0000-0000-0000-000000000001', 'a0000000-0000-0000-0000-000000000010', 9, 'B', 'a0000000-0000-0000-0000-000000000022'),
  ('a0000000-0000-0000-0000-000000000042', 'a0000000-0000-0000-0000-000000000001', 'a0000000-0000-0000-0000-000000000010', 10, 'A', 'a0000000-0000-0000-0000-000000000023'),
  ('a0000000-0000-0000-0000-000000000043', 'a0000000-0000-0000-0000-000000000001', 'a0000000-0000-0000-0000-000000000010', 10, 'B', 'a0000000-0000-0000-0000-000000000024');

-- Base Students
INSERT INTO students (id, school_id, class_id, admission_number, roll_number, full_name, gender, guardian_name, guardian_phone, is_active) VALUES
  (gen_random_uuid(), 'a0000000-0000-0000-0000-000000000001', 'a0000000-0000-0000-0000-000000000040', 'ADM2026001', '01', 'Aarav Patel', 'M', 'Ramesh Patel', '+91-9810000001', true),
  (gen_random_uuid(), 'a0000000-0000-0000-0000-000000000001', 'a0000000-0000-0000-0000-000000000040', 'ADM2026002', '02', 'Diya Sharma', 'F', 'Suresh Sharma', '+91-9810000002', true),
  (gen_random_uuid(), 'a0000000-0000-0000-0000-000000000001', 'a0000000-0000-0000-0000-000000000040', 'ADM2026003', '03', 'Kiran Reddy', 'M', 'Vijay Reddy', '+91-9810000003', true),
  (gen_random_uuid(), 'a0000000-0000-0000-0000-000000000001', 'a0000000-0000-0000-0000-000000000040', 'ADM2026004', '04', 'Ananya Singh', 'F', 'Harish Singh', '+91-9810000004', true),
  (gen_random_uuid(), 'a0000000-0000-0000-0000-000000000001', 'a0000000-0000-0000-0000-000000000040', 'ADM2026005', '05', 'Rohan Joshi', 'M', 'Prakash Joshi', '+91-9810000005', true);

-- Note: Use `go run ./cmd/seed` to populate the complete multi-class, multi-month attendance records dataset.
