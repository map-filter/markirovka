package main

import (
	"context"
	"errors"
	"log"
	"net/http"
	"os"
	"os/signal"
	"strconv"
	"strings"
	"syscall"
	"time"

	"markirovka/emulator/internal/server"
	"markirovka/emulator/internal/store"
)

func main() {
	st, err := store.New(store.Config{
		SeedFile:       env("EMU_SEED", "/seed/seed.json"),
		StateFile:      env("EMU_STATE_FILE", "/data/state.json"),
		OrderDelay:     time.Duration(envFloat("EMU_ORDER_DELAY_SEC", 5) * float64(time.Second)),
		DocDelay:       time.Duration(envFloat("EMU_DOC_DELAY_SEC", 2) * float64(time.Second)),
		AnnulDays:      int(envFloat("EMU_ANNUL_DAYS", 30)),
		AnyParticipant: envBool("EMU_ANY_PARTICIPANT", false),
	}, nil)
	if err != nil {
		log.Fatalf("store: %v", err)
	}
	srv := server.New(server.Config{
		AcceptEmulatedSignature: envBool("EMU_ACCEPT_EMULATED_SIGNATURE", true),
		AcceptAnyCMS:            envBool("EMU_ACCEPT_ANY_CMS", false),
		TokenTTL:                time.Duration(envFloat("EMU_TOKEN_TTL_HOURS", 10) * float64(time.Hour)),
	}, st, nil)

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	go srv.Run(ctx, 500*time.Millisecond)

	addr := env("EMU_ADDR", ":8080")
	hs := &http.Server{Addr: addr, Handler: srv, ReadHeaderTimeout: 10 * time.Second}
	go func() {
		<-ctx.Done()
		shutdown, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		_ = hs.Shutdown(shutdown)
	}()
	log.Printf("ИС МПТ emulator listening on %s, base path %s", addr, server.BasePath)
	if err := hs.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
		log.Fatal(err)
	}
}

func env(key, def string) string {
	if v := strings.TrimSpace(os.Getenv(key)); v != "" {
		return v
	}
	return def
}

func envBool(key string, def bool) bool {
	v, err := strconv.ParseBool(env(key, strconv.FormatBool(def)))
	if err != nil {
		log.Fatalf("%s: %v", key, err)
	}
	return v
}

func envFloat(key string, def float64) float64 {
	v, err := strconv.ParseFloat(env(key, strconv.FormatFloat(def, 'f', -1, 64)), 64)
	if err != nil || v < 0 {
		log.Fatalf("%s: invalid value", key)
	}
	return v
}
