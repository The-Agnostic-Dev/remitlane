// Command remitlane runs the Remitlane server.
//
//	remitlane            serve the API, console and worker (same as "serve")
//	remitlane migrate    apply database migrations and exit
//	remitlane seed       reset the database to the demo data and exit
//	remitlane healthcheck  exit 0 if the local server answers /healthz
package main

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/remitlane/remitlane/internal/app"
	"github.com/remitlane/remitlane/internal/config"
	"github.com/remitlane/remitlane/internal/migrate"
	"github.com/remitlane/remitlane/internal/obs"
	"github.com/remitlane/remitlane/internal/store"
)

func main() {
	cmd := "serve"
	if len(os.Args) > 1 {
		cmd = os.Args[1]
	}
	cfg, err := config.Load()
	if err != nil {
		fmt.Fprintln(os.Stderr, "config:", err)
		os.Exit(2)
	}
	log := obs.NewLogger(os.Stdout, cfg.LogLevel, cfg.LogFormat)
	slog.SetDefault(log)

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	switch cmd {
	case "serve":
		err = serve(ctx, cfg, log)
	case "migrate":
		err = runMigrate(ctx, cfg, log)
	case "seed":
		cfg.SeedOnStart = true
		_, err = app.New(ctx, cfg, log)
	case "healthcheck":
		err = healthcheck(cfg.Port)
	default:
		err = fmt.Errorf("unknown command %q (want serve, migrate, seed or healthcheck)", cmd)
	}
	if err != nil {
		log.Error("exit", "cmd", cmd, "err", err)
		os.Exit(1)
	}
}

func serve(ctx context.Context, cfg config.Config, log *slog.Logger) error {
	a, err := app.New(ctx, cfg, log)
	if err != nil {
		return err
	}
	return a.Run(ctx)
}

func runMigrate(ctx context.Context, cfg config.Config, log *slog.Logger) error {
	st, err := store.Open(ctx, cfg.DatabaseURL, 2)
	if err != nil {
		return err
	}
	defer st.Close()
	return migrate.Up(ctx, st.Pool, log)
}

func healthcheck(port int) error {
	c := &http.Client{Timeout: 3 * time.Second}
	resp, err := c.Get(fmt.Sprintf("http://127.0.0.1:%d/healthz", port))
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("healthz answered %d", resp.StatusCode)
	}
	return nil
}
