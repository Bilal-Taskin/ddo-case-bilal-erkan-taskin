-- 004_turnstile_and_access.sql
-- FitGiriş Turnike Tanımları ve Giriş/Çıkış Erişim Kayıtları

CREATE TABLE IF NOT EXISTS turnstiles (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    branch_id UUID NOT NULL REFERENCES branches(id) ON DELETE CASCADE,
    code VARCHAR(30) NOT NULL,
    name VARCHAR(100) NOT NULL,
    direction VARCHAR(10) NOT NULL CHECK (direction IN ('IN', 'OUT')),
    is_active BOOLEAN NOT NULL DEFAULT true,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CONSTRAINT unique_branch_turnstile_code UNIQUE (branch_id, code)
);

CREATE TABLE IF NOT EXISTS access_logs (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    member_id UUID NOT NULL REFERENCES users(id),
    turnstile_id UUID NOT NULL REFERENCES turnstiles(id),
    branch_id UUID NOT NULL REFERENCES branches(id),
    direction VARCHAR(10) NOT NULL CHECK (direction IN ('IN', 'OUT')),
    status VARCHAR(30) NOT NULL CHECK (status IN ('GRANTED', 'DENIED')),
    reason VARCHAR(100) NOT NULL DEFAULT 'SUCCESS',
    offline_synced BOOLEAN NOT NULL DEFAULT false,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- Fast Index for Anti-Passback check & latency under 50ms:
-- Queries need: SELECT created_at, direction FROM access_logs WHERE member_id = $1 AND branch_id = $2 AND status = 'GRANTED' ORDER BY created_at DESC LIMIT 1
CREATE INDEX IF NOT EXISTS idx_access_logs_anti_passback ON access_logs(member_id, branch_id, status, created_at DESC);
CREATE INDEX IF NOT EXISTS idx_access_logs_branch_hourly ON access_logs(branch_id, created_at);
