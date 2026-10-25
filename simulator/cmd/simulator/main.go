package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"math/rand"
	"net/http"
	"os"
	"os/signal"
	"strconv"
	"syscall"
	"time"
)

type Config struct {
	BackendURL      string
	SeedValue       int64
	TickIntervalSec int
}

func loadConfig() Config {
	url := os.Getenv("BACKEND_URL")
	if url == "" {
		url = "http://localhost:8080"
	}

	seedVal := int64(42)
	if s := os.Getenv("SIMULATOR_SEED"); s != "" {
		if v, err := strconv.ParseInt(s, 10, 64); err == nil {
			seedVal = v
		}
	}

	interval := 3
	if s := os.Getenv("SIMULATOR_INTERVAL_SEC"); s != "" {
		if v, err := strconv.Atoi(s); err == nil && v > 0 {
			interval = v
		}
	}

	return Config{
		BackendURL:      url,
		SeedValue:       seedVal,
		TickIntervalSec: interval,
	}
}

type TokenResponse struct {
	Data struct {
		Tokens struct {
			AccessToken string `json:"access_token"`
		} `json:"tokens"`
		User struct {
			ID    string `json:"id"`
			Email string `json:"email"`
			Role  string `json:"role"`
		} `json:"user"`
	} `json:"data"`
}

type TurnstileItem struct {
	ID        string `json:"id"`
	BranchID  string `json:"branch_id"`
	Code      string `json:"code"`
	Name      string `json:"name"`
	Direction string `json:"direction"`
}

type TurnstileListResponse struct {
	Data []TurnstileItem `json:"data"`
}

type ScheduleItem struct {
	ID        string `json:"id"`
	ClassID   string `json:"class_id"`
	BranchID  string `json:"branch_id"`
	Capacity  int    `json:"capacity"`
	Booked    int    `json:"booked_count"`
	StartTime string `json:"start_time"`
}

type ScheduleListResponse struct {
	Data []ScheduleItem `json:"data"`
}

func main() {
	logger := slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelInfo}))
	slog.SetDefault(logger)

	cfg := loadConfig()
	slog.Info("FitGiris Olay ve Veri Simülatoru baslatildi",
		"backend_url", cfg.BackendURL,
		"seed", cfg.SeedValue,
		"interval_sec", cfg.TickIntervalSec,
	)

	rng := rand.New(rand.NewSource(cfg.SeedValue))
	httpClient := &http.Client{Timeout: 5 * time.Second}

	// 1. Wait for Backend /health to be ready
	waitForBackend(httpClient, cfg.BackendURL)

	// 2. Authenticate Demo Users
	receptionToken, err := login(httpClient, cfg.BackendURL, "reception@fitgiris.com", "Password123!")
	if err != nil {
		slog.Error("Resepsiyon demo girisi basarisiz", "error", err)
		os.Exit(1)
	}

	managerToken, err := login(httpClient, cfg.BackendURL, "manager@fitgiris.com", "Password123!")
	if err != nil {
		slog.Error("Mudur demo girisi basarisiz", "error", err)
		os.Exit(1)
	}

	memberToken, err := login(httpClient, cfg.BackendURL, "member@fitgiris.com", "Password123!")
	if err != nil {
		slog.Error("Uye demo girisi basarisiz", "error", err)
		os.Exit(1)
	}
	slog.Info("Simülatör auth oturumlari basariyla acildi (Reception, Manager, Member)")

	// 3. Fetch initial turnstiles & schedules
	turnstiles := fetchTurnstiles(httpClient, cfg.BackendURL, receptionToken)
	schedules := fetchSchedules(httpClient, cfg.BackendURL, receptionToken)

	slog.Info("Simülatör hedefleri yuklendi", "turnstile_sayisi", len(turnstiles), "schedule_sayisi", len(schedules))

	// 4. Setup graceful shutdown
	shutdown := make(chan os.Signal, 1)
	signal.Notify(shutdown, os.Interrupt, syscall.SIGTERM)

	ticker := time.NewTicker(time.Duration(cfg.TickIntervalSec) * time.Second)
	defer ticker.Stop()

	renewalTicker := time.NewTicker(20 * time.Second)
	defer renewalTicker.Stop()

	tickCount := 0

	for {
		select {
		case sig := <-shutdown:
			slog.Info("Simülator guvenli sekilde kapatiliyor", "signal", sig.String())
			return

		case <-renewalTicker.C:
			// Trigger periodic auto-renewal process simulation
			processAutoRenewals(httpClient, cfg.BackendURL, managerToken)

		case <-ticker.C:
			tickCount++

			// Simulate Turnstile Passage Event
			if len(turnstiles) > 0 {
				t := turnstiles[rng.Intn(len(turnstiles))]
				memberNum := rng.Intn(20000) + 1
				memberCard := fmt.Sprintf("CARD-%05d", memberNum)

				// 10% chance use the active demo member
				if rng.Intn(10) == 0 {
					memberCard = "CARD-DEMO-001"
				}

				simulateTurnstileAccess(httpClient, cfg.BackendURL, receptionToken, t, memberCard)
			}

			// Simulate Group Class Booking Rush Event every 3 ticks
			if tickCount%3 == 0 && len(schedules) > 0 {
				sch := schedules[rng.Intn(len(schedules))]
				simulateClassBooking(httpClient, cfg.BackendURL, memberToken, sch.ID)
			}
		}
	}
}

func waitForBackend(client *http.Client, baseURL string) {
	healthURL := baseURL + "/health"
	slog.Info("Backend baglantisi bekleniyor...", "url", healthURL)

	for {
		resp, err := client.Get(healthURL)
		if err == nil && resp.StatusCode == http.StatusOK {
			resp.Body.Close()
			slog.Info("Backend hazir ve saglikli")
			return
		}
		if resp != nil {
			resp.Body.Close()
		}
		time.Sleep(2 * time.Second)
	}
}

func login(client *http.Client, baseURL, email, password string) (string, error) {
	payload, _ := json.Marshal(map[string]string{
		"email":    email,
		"password": password,
	})

	resp, err := client.Post(baseURL+"/api/v1/auth/login", "application/json", bytes.NewBuffer(payload))
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return "", fmt.Errorf("login failed status %d: %s", resp.StatusCode, string(body))
	}

	var tr TokenResponse
	if err := json.NewDecoder(resp.Body).Decode(&tr); err != nil {
		return "", err
	}
	return tr.Data.Tokens.AccessToken, nil
}

func fetchTurnstiles(client *http.Client, baseURL, token string) []TurnstileItem {
	req, _ := http.NewRequest("GET", baseURL+"/api/v1/turnstiles", nil)
	req.Header.Set("Authorization", "Bearer "+token)

	resp, err := client.Do(req)
	if err != nil || resp.StatusCode != http.StatusOK {
		if resp != nil {
			resp.Body.Close()
		}
		return nil
	}
	defer resp.Body.Close()

	var res TurnstileListResponse
	_ = json.NewDecoder(resp.Body).Decode(&res)
	return res.Data
}

func fetchSchedules(client *http.Client, baseURL, token string) []ScheduleItem {
	req, _ := http.NewRequest("GET", baseURL+"/api/v1/classes/schedules", nil)
	req.Header.Set("Authorization", "Bearer "+token)

	resp, err := client.Do(req)
	if err != nil || resp.StatusCode != http.StatusOK {
		if resp != nil {
			resp.Body.Close()
		}
		return nil
	}
	defer resp.Body.Close()

	var res ScheduleListResponse
	_ = json.NewDecoder(resp.Body).Decode(&res)
	return res.Data
}

func simulateTurnstileAccess(client *http.Client, baseURL, token string, t TurnstileItem, cardToken string) {
	start := time.Now()
	payload, _ := json.Marshal(map[string]interface{}{
		"turnstile_id": t.ID,
		"card_token":   cardToken,
	})

	req, _ := http.NewRequest("POST", baseURL+"/api/v1/turnstiles/access", bytes.NewBuffer(payload))
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Content-Type", "application/json")

	resp, err := client.Do(req)
	duration := time.Since(start)

	if err != nil {
		slog.Warn("Turnike gecis istegi hatasi", "error", err)
		return
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)

	var result struct {
		Data struct {
			Status string `json:"status"`
			Reason string `json:"reason"`
		} `json:"data"`
	}
	_ = json.Unmarshal(body, &result)

	slog.Info("[SIMULATOR TURNSTILE EVENT]",
		"turnstile", t.Code,
		"direction", t.Direction,
		"card", cardToken,
		"status", result.Data.Status,
		"reason", result.Data.Reason,
		"duration_ms", float64(duration.Microseconds())/1000.0,
	)
}

func simulateClassBooking(client *http.Client, baseURL, token, scheduleID string) {
	req, _ := http.NewRequest("POST", baseURL+"/api/v1/classes/schedules/"+scheduleID+"/book", nil)
	req.Header.Set("Authorization", "Bearer "+token)

	resp, err := client.Do(req)
	if err != nil {
		return
	}
	defer resp.Body.Close()

	slog.Info("[SIMULATOR CLASS BOOKING EVENT]",
		"schedule_id", scheduleID,
		"http_status", resp.StatusCode,
	)
}

func processAutoRenewals(client *http.Client, baseURL, token string) {
	req, _ := http.NewRequest("POST", baseURL+"/api/v1/billing/auto-renewals/process", nil)
	req.Header.Set("Authorization", "Bearer "+token)

	resp, err := client.Do(req)
	if err != nil {
		return
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)
	slog.Info("[SIMULATOR AUTO-RENEWAL BATCH PROCESSED]",
		"status", resp.StatusCode,
		"response", string(body),
	)
}
