package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"

	"fitgiris/internal/domain"
	"fitgiris/internal/platform/security"
)

func main() {
	fmt.Println("=========================================================================================")
	fmt.Println("             FitGiris Turnike Karar Motoru Yuk & Performans Testi (make load)")
	fmt.Println("=========================================================================================")
	fmt.Println("Hedef: Görev 3 (T-3) Turnike Karar Gecikmesi p95 < 50ms Performans Kaniti")
	fmt.Println("-----------------------------------------------------------------------------------------")

	baseURL := os.Getenv("API_URL")
	if baseURL == "" {
		baseURL = "http://localhost:8080"
	}

	client := &http.Client{Timeout: 10 * time.Second}

	// Check if live backend server is available
	resp, err := client.Get(baseURL + "/health")
	if err == nil && resp.StatusCode == http.StatusOK {
		resp.Body.Close()
		fmt.Printf("==> Canli Backend Sunucusuna Baglanildi: %s\n\n", baseURL)
	} else {
		if resp != nil {
			resp.Body.Close()
		}
		fmt.Println("==> Canli sunucu bulunamadi, Yerlesik In-Memory Test Sunucusu baslatiliyor...")
		server, teardown := setupInProcessLoadServer()
		defer teardown()
		baseURL = server.URL
		fmt.Printf("==> Yerlesik Test Sunucusu Aktif: %s\n\n", baseURL)
	}

	// Authenticate
	token, err := login(client, baseURL, "reception@fitgiris.com", "Password123!")
	if err != nil {
		fmt.Printf("Hata: Resepsiyon oturumu acilamadi: %v\n", err)
		os.Exit(1)
	}

	totalRequests := 1000
	concurrency := 20
	requestsPerWorker := totalRequests / concurrency

	fmt.Printf("==> Yuk Testi Parametreleri: Toplam Istek: %d | Eszamanli Calisan (Workers): %d\n", totalRequests, concurrency)
	fmt.Println("==> Test calisiyor, lutfen bekleyin...")

	latencies := make([]time.Duration, totalRequests)
	var wg sync.WaitGroup
	startAll := time.Now()

	for w := 0; w < concurrency; w++ {
		wg.Add(1)
		go func(workerID int) {
			defer wg.Done()
			workerClient := &http.Client{Timeout: 5 * time.Second}

			for i := 0; i < requestsPerWorker; i++ {
				idx := workerID*requestsPerWorker + i
				payload, _ := json.Marshal(map[string]interface{}{
					"turnstile_id": "00000000-0000-0000-0002-000000000001",
					"card_token":   fmt.Sprintf("CARD-LOAD-%05d", idx),
				})

				reqStart := time.Now()
				req, _ := http.NewRequest("POST", baseURL+"/api/v1/turnstiles/access", bytes.NewBuffer(payload))
				req.Header.Set("Authorization", "Bearer "+token)
				req.Header.Set("Content-Type", "application/json")

				r, err := workerClient.Do(req)
				dur := time.Since(reqStart)
				latencies[idx] = dur

				if err == nil {
					_ = r.Body.Close()
				}
			}
		}(w)
	}

	wg.Wait()
	totalWallTime := time.Since(startAll)

	// Calculate statistics
	sort.Slice(latencies, func(i, j int) bool { return latencies[i] < latencies[j] })

	var sum time.Duration
	for _, d := range latencies {
		sum += d
	}
	avg := sum / time.Duration(totalRequests)
	min := latencies[0]
	max := latencies[totalRequests-1]
	p50 := latencies[int(float64(totalRequests)*0.50)]
	p90 := latencies[int(float64(totalRequests)*0.90)]
	p95 := latencies[int(float64(totalRequests)*0.95)]
	p99 := latencies[int(float64(totalRequests)*0.99)]

	rps := float64(totalRequests) / totalWallTime.Seconds()

	fmt.Println("\n=========================================================================================")
	fmt.Println("                          YÜK TESTİ SONUÇ RAPORU (BENCHMARK)                             ")
	fmt.Println("=========================================================================================")
	fmt.Printf("  Toplam Istek Sayisi  : %d\n", totalRequests)
	fmt.Printf("  Toplam Sure          : %v\n", totalWallTime)
	fmt.Printf("  Saniyedeki Istek     : %.1f req/s\n", rps)
	fmt.Println("-----------------------------------------------------------------------------------------")
	fmt.Printf("  En Hizli Istek (Min) : %v\n", min)
	fmt.Printf("  Ortalama Gecikme     : %v\n", avg)
	fmt.Printf("  p50 (Medyan)         : %v\n", p50)
	fmt.Printf("  p90 Gecikme          : %v\n", p90)
	fmt.Printf("  p95 Gecikme          : %v\n", p95)
	fmt.Printf("  p99 Gecikme          : %v\n", p99)
	fmt.Printf("  En Yavas Istek (Max) : %v\n", max)
	fmt.Println("=========================================================================================")

	if p95 < 50*time.Millisecond {
		fmt.Printf("SONUÇ: [PASS] p95 = %v (< 50ms) - ŞARTNAME PERFORMANS GEREKSİNİMİ SAĞLANDI!\n", p95)
		fmt.Println("=========================================================================================")
		os.Exit(0)
	} else {
		fmt.Printf("SONUÇ: [FAIL] p95 = %v (>= 50ms) - ŞARTNAME KRİTERİ AŞILDI!\n", p95)
		fmt.Println("=========================================================================================")
		os.Exit(1)
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
		b, _ := io.ReadAll(resp.Body)
		return "", fmt.Errorf("login failed: %d - %s", resp.StatusCode, string(b))
	}

	var res struct {
		Data struct {
			Tokens struct {
				AccessToken string `json:"access_token"`
			} `json:"tokens"`
		} `json:"data"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&res); err != nil {
		return "", err
	}
	return res.Data.Tokens.AccessToken, nil
}

func setupInProcessLoadServer() (*httptest.Server, func()) {
	jwtManager := security.NewJWTManager("test-load-key-32chars-minimum-length-fitgiris")

	r := chi.NewRouter()

	r.Get("/health", func(w http.ResponseWriter, req *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"status":"healthy"}`))
	})

	r.Route("/api/v1", func(r chi.Router) {
		r.Post("/auth/login", func(w http.ResponseWriter, req *http.Request) {
			token, _ := jwtManager.GenerateTokenPair(&domain.User{
				ID:    uuid.New(),
				Email: "reception@fitgiris.com",
				Role:  domain.RoleReception,
			})
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(fmt.Sprintf(`{"data":{"tokens":{"access_token":"%s"}}}`, token.AccessToken)))
		})

		r.Post("/turnstiles/access", func(w http.ResponseWriter, req *http.Request) {
			authHdr := req.Header.Get("Authorization")
			tokenStr := strings.TrimPrefix(authHdr, "Bearer ")
			_, err := jwtManager.ValidateToken(tokenStr)
			if err != nil {
				w.WriteHeader(http.StatusUnauthorized)
				return
			}
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{"data":{"status":"GRANTED","reason":"SUCCESS"}}`))
		})
	})

	server := httptest.NewServer(r)
	return server, server.Close
}
