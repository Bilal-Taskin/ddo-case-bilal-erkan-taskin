package main

import (
	"log/slog"
	"os"
	"os/signal"
	"syscall"
	"time"
)

func main() {
	logger := slog.New(slog.NewJSONHandler(os.Stdout, nil))
	slog.SetDefault(logger)

	slog.Info("FitGiris Simulator servisi baslatildi (Phase 1 hazirlik modu)")

	shutdown := make(chan os.Signal, 1)
	signal.Notify(shutdown, os.Interrupt, syscall.SIGTERM)

	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			slog.Info("FitGiris Simulator saglikli calisiyor")
		case sig := <-shutdown:
			slog.Info("Simulator durduruluyor", "signal", sig.String())
			return
		}
	}
}
