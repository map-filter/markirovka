package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"io"
	"log/slog"
	"os"
	"os/signal"
	"strconv"
	"strings"
	"syscall"
	"time"

	"markirovka/agent/internal/agent"
	"markirovka/agent/internal/client"
	"markirovka/agent/internal/label"
	"markirovka/agent/internal/printer"
)

var version = "dev"

const usage = `markirovka-agent: print agent for GS1 Data Matrix labels

Usage:
  markirovka-agent [run] [flags]     take print jobs from the server (default)
  markirovka-agent test-print [flags] print a built-in test ZPL label to PRINTER
  markirovka-agent version            print version

Run "markirovka-agent <command> -h" for flags. Every flag has an env fallback.
`

func main() {
	if err := run(os.Args[1:], os.Stdout, os.Stderr); err != nil {
		if !errors.Is(err, flag.ErrHelp) {
			fmt.Fprintln(os.Stderr, "error:", err)
		}
		os.Exit(1)
	}
}

func run(args []string, stdout, stderr io.Writer) error {
	cmd := "run"
	if len(args) > 0 && !strings.HasPrefix(args[0], "-") {
		cmd, args = args[0], args[1:]
	}
	switch cmd {
	case "run":
		return runAgent(args, stderr)
	case "test-print":
		return testPrint(args, stdout, stderr)
	case "version":
		fmt.Fprintln(stdout, version)
		return nil
	case "help":
		fmt.Fprint(stdout, usage)
		return nil
	default:
		fmt.Fprint(stderr, usage)
		return fmt.Errorf("unknown command %q", cmd)
	}
}

type config struct {
	serverURL     string
	token         string
	printer       string
	pollWait      time.Duration
	insecureTLS   bool
	allowOverride bool
	logLevel      string
}

func parseConfig(name string, args []string, stderr io.Writer) (config, error) {
	var c config
	fs := flag.NewFlagSet(name, flag.ContinueOnError)
	fs.SetOutput(stderr)
	fs.StringVar(&c.serverURL, "server-url", env("SERVER_URL", ""), "server base URL, e.g. https://label.example.kz (env SERVER_URL)")
	fs.StringVar(&c.token, "token", env("AGENT_TOKEN", ""), "agent token (env AGENT_TOKEN, preferred over the flag)")
	fs.StringVar(&c.printer, "printer", env("PRINTER", ""), "printer URI: tcp://host:9100, cups://queue, file:///dir (env PRINTER)")
	waitSec := fs.Int("poll-wait", envInt("POLL_WAIT", 20), "long-poll wait in seconds (env POLL_WAIT)")
	fs.BoolVar(&c.insecureTLS, "insecure-tls", envBool("INSECURE_TLS", false), "skip TLS certificate verification, dev only (env INSECURE_TLS)")
	fs.BoolVar(&c.allowOverride, "allow-printer-override", envBool("ALLOW_PRINTER_OVERRIDE", false), "let the server choose another printer per job (env ALLOW_PRINTER_OVERRIDE)")
	fs.StringVar(&c.logLevel, "log-level", env("LOG_LEVEL", "info"), "debug, info, warn or error (env LOG_LEVEL)")
	if err := fs.Parse(args); err != nil {
		return c, err
	}
	if fs.NArg() > 0 {
		return c, fmt.Errorf("unexpected arguments: %v", fs.Args())
	}
	if *waitSec < 1 || *waitSec > 300 {
		return c, fmt.Errorf("poll-wait must be between 1 and 300 seconds, got %d", *waitSec)
	}
	c.pollWait = time.Duration(*waitSec) * time.Second
	return c, nil
}

func runAgent(args []string, stderr io.Writer) error {
	cfg, err := parseConfig("run", args, stderr)
	if err != nil {
		return err
	}
	logger, err := newLogger(cfg.logLevel, stderr)
	if err != nil {
		return err
	}
	if cfg.serverURL == "" {
		return errors.New("SERVER_URL is required")
	}
	p, err := printer.Open(cfg.printer)
	if err != nil {
		return fmt.Errorf("PRINTER: %w", err)
	}
	c, err := client.New(client.Options{
		BaseURL:     cfg.serverURL,
		Token:       cfg.token,
		UserAgent:   "markirovka-agent/" + version,
		InsecureTLS: cfg.insecureTLS,
	})
	if err != nil {
		return err
	}
	if strings.HasPrefix(strings.ToLower(cfg.serverURL), "http://") {
		logger.Warn("server url is not https, the agent token is sent in clear text")
	}
	if cfg.insecureTLS {
		logger.Warn("TLS certificate verification is disabled (INSECURE_TLS=true)")
	}
	hostname, _ := os.Hostname()

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	return agent.New(agent.Config{
		Server:        c,
		Printer:       p,
		AllowOverride: cfg.allowOverride,
		PollWait:      cfg.pollWait,
		Version:       version,
		Hostname:      hostname,
		Logger:        logger,
	}).Run(ctx)
}

func testPrint(args []string, stdout, stderr io.Writer) error {
	cfg, err := parseConfig("test-print", args, stderr)
	if err != nil {
		return err
	}
	p, err := printer.Open(cfg.printer)
	if err != nil {
		return fmt.Errorf("PRINTER: %w", err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
	defer cancel()
	job := printer.Job{ID: "test-print", Format: printer.FormatZPL, Copies: 1, Data: label.TestZPL()}
	if err := p.Print(ctx, job); err != nil {
		return err
	}
	fmt.Fprintf(stdout, "test label sent to %s\n", p.URI())
	return nil
}

func newLogger(level string, w io.Writer) (*slog.Logger, error) {
	var l slog.Level
	if err := l.UnmarshalText([]byte(level)); err != nil {
		return nil, fmt.Errorf("LOG_LEVEL: %w", err)
	}
	return slog.New(slog.NewTextHandler(w, &slog.HandlerOptions{Level: l})), nil
}

func env(key, def string) string {
	if v, ok := os.LookupEnv(key); ok && v != "" {
		return v
	}
	return def
}

func envInt(key string, def int) int {
	if n, err := strconv.Atoi(env(key, "")); err == nil {
		return n
	}
	return def
}

func envBool(key string, def bool) bool {
	if b, err := strconv.ParseBool(env(key, "")); err == nil {
		return b
	}
	return def
}
