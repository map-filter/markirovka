package agent

import (
	"context"
	"encoding/base64"
	"errors"
	"fmt"
	"log/slog"
	"time"

	"markirovka/agent/internal/client"
	"markirovka/agent/internal/printer"
)

const (
	StatusPrinted = "printed"
	StatusFailed  = "failed"
	maxCopies     = 1000
)

type Server interface {
	NextJob(ctx context.Context, wait time.Duration) (*client.Job, error)
	Ack(ctx context.Context, jobID string, ack client.Ack) error
	Heartbeat(ctx context.Context, hb client.Heartbeat) error
}

type Config struct {
	Server            Server
	Printer           printer.Printer
	OpenPrinter       func(uri string) (printer.Printer, error)
	AllowOverride     bool
	PollWait          time.Duration
	HeartbeatInterval time.Duration
	IdleDelay         time.Duration
	JobTimeout        time.Duration
	Backoff           Backoff
	AckRetries        int
	Version           string
	Hostname          string
	Logger            *slog.Logger
}

type Agent struct {
	cfg Config
	log *slog.Logger
}

func New(cfg Config) *Agent {
	if cfg.OpenPrinter == nil {
		cfg.OpenPrinter = printer.Open
	}
	if cfg.PollWait <= 0 {
		cfg.PollWait = 20 * time.Second
	}
	if cfg.HeartbeatInterval <= 0 {
		cfg.HeartbeatInterval = 30 * time.Second
	}
	if cfg.IdleDelay <= 0 {
		cfg.IdleDelay = time.Second
	}
	if cfg.JobTimeout <= 0 {
		cfg.JobTimeout = 2 * time.Minute
	}
	if cfg.Backoff.Min <= 0 {
		cfg.Backoff.Min = time.Second
	}
	if cfg.Backoff.Max <= 0 {
		cfg.Backoff.Max = time.Minute
	}
	if cfg.AckRetries <= 0 {
		cfg.AckRetries = 5
	}
	if cfg.Logger == nil {
		cfg.Logger = slog.Default()
	}
	return &Agent{cfg: cfg, log: cfg.Logger}
}

func (a *Agent) Run(ctx context.Context) error {
	a.log.Info("agent started", "version", a.cfg.Version, "printer", a.cfg.Printer.URI(), "override_allowed", a.cfg.AllowOverride)

	hbDone := make(chan struct{})
	go func() {
		defer close(hbDone)
		a.heartbeatLoop(ctx)
	}()

	a.pollLoop(ctx)
	<-hbDone
	a.log.Info("agent stopped")
	return nil
}

func (a *Agent) heartbeatLoop(ctx context.Context) {
	ticker := time.NewTicker(a.cfg.HeartbeatInterval)
	defer ticker.Stop()
	for {
		hb := client.Heartbeat{Version: a.cfg.Version, Hostname: a.cfg.Hostname, Printer: a.cfg.Printer.URI()}
		if err := a.cfg.Server.Heartbeat(ctx, hb); err != nil && ctx.Err() == nil {
			a.log.Warn("heartbeat failed", "err", err)
		}
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
		}
	}
}

func (a *Agent) pollLoop(ctx context.Context) {
	backoff := a.cfg.Backoff
	for ctx.Err() == nil {
		started := time.Now()
		job, err := a.cfg.Server.NextJob(ctx, a.cfg.PollWait)
		if err != nil {
			if ctx.Err() != nil {
				return
			}
			delay := backoff.Next()
			if errors.Is(err, client.ErrUnauthorized) {
				a.log.Error("server rejected agent token, check AGENT_TOKEN", "retry_in", delay)
			} else {
				a.log.Warn("poll failed", "err", err, "retry_in", delay)
			}
			sleep(ctx, delay)
			continue
		}
		backoff.Reset()
		if job == nil {
			if time.Since(started) < a.cfg.IdleDelay {
				sleep(ctx, a.cfg.IdleDelay)
			}
			continue
		}
		a.handle(ctx, job)
	}
}

func (a *Agent) handle(ctx context.Context, job *client.Job) {
	jobCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), a.cfg.JobTimeout)
	defer cancel()

	log := a.log.With("job", job.ID, "format", job.Format, "copies", job.Copies)
	log.Info("job received")

	ack := client.Ack{Status: StatusPrinted}
	if target, err := a.print(jobCtx, job); err != nil {
		log.Error("job failed", "err", err)
		ack = client.Ack{Status: StatusFailed, Error: err.Error()}
	} else {
		log.Info("job printed", "printer", target)
	}
	a.ack(jobCtx, log, job.ID, ack)
}

func (a *Agent) print(ctx context.Context, job *client.Job) (string, error) {
	data, err := base64.StdEncoding.DecodeString(job.PayloadBase64)
	if err != nil {
		return "", fmt.Errorf("decode payload_base64: %w", err)
	}
	if job.Copies > maxCopies {
		return "", fmt.Errorf("copies %d exceeds limit %d", job.Copies, maxCopies)
	}
	pj := printer.Job{ID: job.ID, Format: job.Format, Copies: job.Copies, Data: data}
	if err := printer.Validate(pj); err != nil {
		return "", err
	}
	target, err := a.target(job.Printer)
	if err != nil {
		return "", err
	}
	if err := target.Print(ctx, pj); err != nil {
		return target.URI(), err
	}
	return target.URI(), nil
}

func (a *Agent) target(override string) (printer.Printer, error) {
	if override == "" || override == a.cfg.Printer.URI() {
		return a.cfg.Printer, nil
	}
	if !a.cfg.AllowOverride {
		return nil, fmt.Errorf("job requests printer %q but printer override is disabled on this agent (ALLOW_PRINTER_OVERRIDE=false)", override)
	}
	p, err := a.cfg.OpenPrinter(override)
	if err != nil {
		return nil, fmt.Errorf("open override printer: %w", err)
	}
	return p, nil
}

func (a *Agent) ack(ctx context.Context, log *slog.Logger, jobID string, ack client.Ack) {
	backoff := a.cfg.Backoff
	for attempt := 1; ; attempt++ {
		err := a.cfg.Server.Ack(ctx, jobID, ack)
		if err == nil {
			log.Debug("ack sent", "status", ack.Status)
			return
		}
		if attempt >= a.cfg.AckRetries || ctx.Err() != nil {
			log.Error("ack failed, giving up", "status", ack.Status, "attempts", attempt, "err", err)
			return
		}
		delay := backoff.Next()
		log.Warn("ack failed", "err", err, "retry_in", delay)
		sleep(ctx, delay)
	}
}

func sleep(ctx context.Context, d time.Duration) {
	t := time.NewTimer(d)
	defer t.Stop()
	select {
	case <-ctx.Done():
	case <-t.C:
	}
}
