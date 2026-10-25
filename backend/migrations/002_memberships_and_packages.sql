-- 002_memberships_and_packages.sql
-- FitGiriş Üyelik Paketleri, Üye Üyelikleri, Dondurma ve Devir Tabloları

CREATE TABLE IF NOT EXISTS package_types (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    name VARCHAR(100) NOT NULL,
    type VARCHAR(30) NOT NULL CHECK (type IN ('DURATION', 'SESSION')),
    duration_months INT NOT NULL DEFAULT 1,
    total_entries INT NOT NULL DEFAULT 0,
    price_kurus BIGINT NOT NULL CHECK (price_kurus >= 0),
    all_branches BOOLEAN NOT NULL DEFAULT false,
    max_freeze_days INT NOT NULL DEFAULT 30,
    is_active BOOLEAN NOT NULL DEFAULT true,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TABLE IF NOT EXISTS member_packages (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    member_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    package_type_id UUID NOT NULL REFERENCES package_types(id),
    branch_id UUID NOT NULL REFERENCES branches(id),
    status VARCHAR(30) NOT NULL CHECK (status IN ('PENDING_PAYMENT', 'ACTIVE', 'FROZEN', 'TRANSFERRED', 'SUSPENDED', 'EXPIRED', 'CANCELLED')),
    start_date DATE NOT NULL,
    end_date DATE NOT NULL,
    remaining_entries INT NOT NULL DEFAULT 0,
    total_entries INT NOT NULL DEFAULT 0,
    price_paid_kurus BIGINT NOT NULL CHECK (price_paid_kurus >= 0),
    auto_renew BOOLEAN NOT NULL DEFAULT false,
    card_token VARCHAR(255),
    consecutive_failed_attempts INT NOT NULL DEFAULT 0,
    next_retry_date DATE,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TABLE IF NOT EXISTS package_freezes (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    member_package_id UUID NOT NULL REFERENCES member_packages(id) ON DELETE CASCADE,
    freeze_start_date DATE NOT NULL,
    freeze_end_date DATE NOT NULL,
    actual_unfreeze_date DATE,
    days_frozen INT NOT NULL DEFAULT 0,
    reason TEXT,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TABLE IF NOT EXISTS package_transfers (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    from_member_id UUID NOT NULL REFERENCES users(id),
    to_member_id UUID NOT NULL REFERENCES users(id),
    source_package_id UUID NOT NULL REFERENCES member_packages(id),
    new_package_id UUID NOT NULL REFERENCES member_packages(id),
    transferred_days INT NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_member_packages_member ON member_packages(member_id);
CREATE INDEX IF NOT EXISTS idx_member_packages_status ON member_packages(status);
CREATE INDEX IF NOT EXISTS idx_member_packages_active ON member_packages(member_id, status, end_date);
CREATE INDEX IF NOT EXISTS idx_package_freezes_package ON package_freezes(member_package_id);
