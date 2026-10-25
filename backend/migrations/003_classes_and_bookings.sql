-- 003_classes_and_bookings.sql
-- FitGiriş Dersler, Ders Programı, Rezervasyonlar ve Bekleme Listesi

CREATE TABLE IF NOT EXISTS classes (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    name VARCHAR(100) NOT NULL,
    description TEXT,
    duration_minutes INT NOT NULL DEFAULT 50,
    default_capacity INT NOT NULL DEFAULT 20,
    is_active BOOLEAN NOT NULL DEFAULT true,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TABLE IF NOT EXISTS class_schedules (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    class_id UUID NOT NULL REFERENCES classes(id) ON DELETE RESTRICT,
    branch_id UUID NOT NULL REFERENCES branches(id) ON DELETE RESTRICT,
    trainer_id UUID NOT NULL REFERENCES users(id) ON DELETE RESTRICT,
    start_time TIMESTAMPTZ NOT NULL,
    end_time TIMESTAMPTZ NOT NULL,
    capacity INT NOT NULL CHECK (capacity > 0),
    booked_count INT NOT NULL DEFAULT 0 CHECK (booked_count >= 0 AND booked_count <= capacity),
    status VARCHAR(30) NOT NULL DEFAULT 'SCHEDULED' CHECK (status IN ('SCHEDULED', 'CANCELLED', 'COMPLETED')),
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CONSTRAINT check_schedule_times CHECK (end_time > start_time)
);

CREATE TABLE IF NOT EXISTS class_bookings (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    schedule_id UUID NOT NULL REFERENCES class_schedules(id) ON DELETE CASCADE,
    member_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    status VARCHAR(30) NOT NULL DEFAULT 'CONFIRMED' CHECK (status IN ('CONFIRMED', 'CANCELLED', 'LATE_CANCELLED', 'ATTENDED', 'NO_SHOW')),
    cancelled_at TIMESTAMPTZ,
    penalty_applied BOOLEAN NOT NULL DEFAULT false,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CONSTRAINT unique_schedule_member_booking UNIQUE (schedule_id, member_id)
);

CREATE TABLE IF NOT EXISTS class_waitlist (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    schedule_id UUID NOT NULL REFERENCES class_schedules(id) ON DELETE CASCADE,
    member_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    status VARCHAR(30) NOT NULL DEFAULT 'WAITING' CHECK (status IN ('WAITING', 'PROMOTED', 'EXPIRED', 'CANCELLED')),
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    promoted_at TIMESTAMPTZ,
    CONSTRAINT unique_schedule_member_waitlist UNIQUE (schedule_id, member_id)
);

CREATE INDEX IF NOT EXISTS idx_class_schedules_branch_time ON class_schedules(branch_id, start_time);
CREATE INDEX IF NOT EXISTS idx_class_schedules_trainer_time ON class_schedules(trainer_id, start_time);
CREATE INDEX IF NOT EXISTS idx_class_bookings_member ON class_bookings(member_id, status);
CREATE INDEX IF NOT EXISTS idx_class_bookings_schedule ON class_bookings(schedule_id, status);
CREATE INDEX IF NOT EXISTS idx_class_waitlist_fifo ON class_waitlist(schedule_id, status, created_at ASC);
