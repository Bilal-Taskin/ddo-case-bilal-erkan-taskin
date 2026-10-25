package service

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"
)

type ChargeRequest struct {
	MemberID       uuid.UUID
	AmountKurus    int64
	CardToken      string
	IdempotencyKey string
	Description    string
}

type ChargeResult struct {
	Success      bool
	ProviderTxID string
	ErrorCode    string
	ErrorMessage string
}

type PaymentProvider interface {
	Charge(ctx context.Context, req ChargeRequest) (*ChargeResult, error)
}

// MockPaymentProvider simulates real-world payment gateway responses,
// latency, network timeouts, and configurable failure scenarios.
type MockPaymentProvider struct {
	mu           sync.RWMutex
	failNextN    int
	forcedError  string
	deterministicFailures map[string]string // map cardToken -> error code
}

func NewMockPaymentProvider() *MockPaymentProvider {
	return &MockPaymentProvider{
		deterministicFailures: make(map[string]string),
	}
}

func (p *MockPaymentProvider) SetFailNextN(n int, errorCode string) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.failNextN = n
	p.forcedError = errorCode
}

func (p *MockPaymentProvider) SetCardFailure(cardToken, errorCode string) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.deterministicFailures[cardToken] = errorCode
}

func (p *MockPaymentProvider) Charge(ctx context.Context, req ChargeRequest) (*ChargeResult, error) {
	p.mu.Lock()
	defer p.mu.Unlock()

	// 1. Check card token failure prefixes
	if strings.HasPrefix(req.CardToken, "FAIL_TIMEOUT") {
		return &ChargeResult{
			Success:      false,
			ErrorCode:    "ERR_BANK_TIMEOUT",
			ErrorMessage: "Banka baglanti zaman asimina ugradi",
		}, nil
	}
	if strings.HasPrefix(req.CardToken, "FAIL_INSUFFICIENT") {
		return &ChargeResult{
			Success:      false,
			ErrorCode:    "ERR_INSUFFICIENT_FUNDS",
			ErrorMessage: "Kart bakiyesi yetersiz",
		}, nil
	}
	if strings.HasPrefix(req.CardToken, "FAIL_EXPIRED") {
		return &ChargeResult{
			Success:      false,
			ErrorCode:    "ERR_CARD_EXPIRED",
			ErrorMessage: "Kredi kartinin son kullanma tarihi gecmis",
		}, nil
	}
	if strings.HasPrefix(req.CardToken, "FAIL") {
		return &ChargeResult{
			Success:      false,
			ErrorCode:    "ERR_CARD_DECLINED",
			ErrorMessage: "Kart banka tarafindan reddedildi",
		}, nil
	}

	// 2. Check deterministic failure map
	if errCode, exists := p.deterministicFailures[req.CardToken]; exists {
		return &ChargeResult{
			Success:      false,
			ErrorCode:    errCode,
			ErrorMessage: fmt.Sprintf("Simule edilmis banka hatasi: %s", errCode),
		}, nil
	}

	// 3. Check programmed transient failures
	if p.failNextN > 0 {
		p.failNextN--
		errCode := p.forcedError
		if errCode == "" {
			errCode = "ERR_PAYMENT_FAILED"
		}
		return &ChargeResult{
			Success:      false,
			ErrorCode:    errCode,
			ErrorMessage: "Gecici banka provizyon hatasi",
		}, nil
	}

	// 4. Default: Success
	txID := fmt.Sprintf("tx_mock_%d_%s", time.Now().UnixNano(), uuid.New().String()[:8])
	return &ChargeResult{
		Success:      true,
		ProviderTxID: txID,
	}, nil
}
