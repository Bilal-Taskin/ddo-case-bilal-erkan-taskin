-- 005_billing_and_invoices.sql
-- FitGiriş Faturalar, Sıralı Boşluksuz Sayaç, Ödemeler ve Otomatik Yenileme Kuyruğu

CREATE TABLE IF NOT EXISTS invoice_sequences (
    year INT NOT NULL,
    branch_id UUID NOT NULL REFERENCES branches(id) ON DELETE CASCADE,
    current_val BIGINT NOT NULL DEFAULT 0,
    PRIMARY KEY (year, branch_id)
);

CREATE TABLE IF NOT EXISTS invoices (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    invoice_number VARCHAR(50) NOT NULL UNIQUE,
    member_id UUID NOT NULL REFERENCES users(id),
    branch_id UUID NOT NULL REFERENCES branches(id),
    member_package_id UUID REFERENCES member_packages(id) ON DELETE SET NULL,
    year INT NOT NULL,
    sequence_number BIGINT NOT NULL,
    amount_kurus BIGINT NOT NULL CHECK (amount_kurus >= 0),
    tax_kurus BIGINT NOT NULL DEFAULT 0 CHECK (tax_kurus >= 0),
    status VARCHAR(30) NOT NULL CHECK (status IN ('PAID', 'REFUNDED', 'CANCELLED', 'PENDING')),
    payment_method VARCHAR(30) NOT NULL DEFAULT 'CREDIT_CARD',
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CONSTRAINT unique_invoice_sequence_per_branch UNIQUE (year, branch_id, sequence_number)
);

CREATE TABLE IF NOT EXISTS payments (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    invoice_id UUID NOT NULL REFERENCES invoices(id) ON DELETE CASCADE,
    member_id UUID NOT NULL REFERENCES users(id),
    amount_kurus BIGINT NOT NULL CHECK (amount_kurus >= 0),
    status VARCHAR(30) NOT NULL CHECK (status IN ('SUCCESS', 'FAILED', 'PENDING')),
    provider_tx_id VARCHAR(100),
    error_code VARCHAR(50),
    error_message TEXT,
    idempotency_key VARCHAR(255) UNIQUE,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TABLE IF NOT EXISTS auto_renewal_queue (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    member_package_id UUID NOT NULL REFERENCES member_packages(id) ON DELETE CASCADE,
    attempt_number INT NOT NULL DEFAULT 1 CHECK (attempt_number BETWEEN 1 AND 4),
    scheduled_for DATE NOT NULL,
    status VARCHAR(30) NOT NULL DEFAULT 'PENDING' CHECK (status IN ('PENDING', 'SUCCESS', 'FAILED', 'ABANDONED')),
    last_attempt_at TIMESTAMPTZ,
    error_reason TEXT,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_invoices_member ON invoices(member_id);
CREATE INDEX IF NOT EXISTS idx_invoices_branch_year ON invoices(branch_id, year, sequence_number);
CREATE INDEX IF NOT EXISTS idx_payments_invoice ON payments(invoice_id);
CREATE INDEX IF NOT EXISTS idx_payments_idempotency ON payments(idempotency_key);
CREATE INDEX IF NOT EXISTS idx_auto_renewal_due ON auto_renewal_queue(status, scheduled_for);
