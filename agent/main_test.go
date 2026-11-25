package main

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"markirovka/agent/internal/label"
)

func TestVersionCommand(t *testing.T) {
	var out bytes.Buffer
	if err := run([]string{"version"}, &out, &out); err != nil {
		t.Fatal(err)
	}
	if strings.TrimSpace(out.String()) != version {
		t.Fatalf("got %q", out.String())
	}
}

func TestUnknownCommand(t *testing.T) {
	var out bytes.Buffer
	if err := run([]string{"bogus"}, &out, &out); err == nil {
		t.Fatal("expected error")
	}
}

func TestTestPrintToFile(t *testing.T) {
	dir := t.TempDir()
	var out bytes.Buffer
	if err := run([]string{"test-print", "-printer", "file://" + filepath.ToSlash(dir)}, &out, &out); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(filepath.Join(dir, "test-print-1.zpl"))
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(data, label.TestZPL()) {
		t.Fatalf("test label differs: %q", data)
	}
}

func TestEnvFallback(t *testing.T) {
	t.Setenv("PRINTER", "tcp://10.0.0.5:9100")
	t.Setenv("POLL_WAIT", "5")
	t.Setenv("ALLOW_PRINTER_OVERRIDE", "true")
	cfg, err := parseConfig("run", nil, &bytes.Buffer{})
	if err != nil {
		t.Fatal(err)
	}
	if cfg.printer != "tcp://10.0.0.5:9100" || cfg.pollWait.Seconds() != 5 || !cfg.allowOverride || cfg.insecureTLS {
		t.Fatalf("cfg %+v", cfg)
	}
	cfg, err = parseConfig("run", []string{"-poll-wait", "30", "-printer", "cups://q"}, &bytes.Buffer{})
	if err != nil {
		t.Fatal(err)
	}
	if cfg.printer != "cups://q" || cfg.pollWait.Seconds() != 30 {
		t.Fatalf("flags must win over env: %+v", cfg)
	}
}

func TestRunRequiresServerURL(t *testing.T) {
	t.Setenv("SERVER_URL", "")
	var out bytes.Buffer
	if err := run([]string{"run", "-printer", "tcp://p:9100"}, &out, &out); err == nil || !strings.Contains(err.Error(), "SERVER_URL") {
		t.Fatalf("err %v", err)
	}
}
