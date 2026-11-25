package agent

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"io"
	"log/slog"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"markirovka/agent/internal/client"
	"markirovka/agent/internal/printer"
)

var gs1ZPL = []byte("^XA^FO20,20^BXN,5,200,0,0,1,~^FD~10104600000000015\x1d21ABC~193TEST^FS^XZ")

type response struct {
	status int
	job    *client.Job
}

type fakeServer struct {
	t         *testing.T
	mu        sync.Mutex
	queue     []response
	polls     []time.Time
	acks      map[string]client.Ack
	ackCh     chan string
	heartbeat chan client.Heartbeat
}

func newFakeServer(t *testing.T, queue ...response) (*fakeServer, *client.Client) {
	fs := &fakeServer{
		t:         t,
		queue:     queue,
		acks:      map[string]client.Ack{},
		ackCh:     make(chan string, 16),
		heartbeat: make(chan client.Heartbeat, 16),
	}
	srv := httptest.NewServer(fs)
	t.Cleanup(srv.Close)
	c, err := client.New(client.Options{BaseURL: srv.URL, Token: "tok"})
	if err != nil {
		t.Fatal(err)
	}
	return fs, c
}

func (fs *fakeServer) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if r.Header.Get("Authorization") != "Bearer tok" {
		w.WriteHeader(http.StatusUnauthorized)
		return
	}
	switch {
	case r.URL.Path == "/api/agent/heartbeat":
		var hb client.Heartbeat
		json.NewDecoder(r.Body).Decode(&hb)
		select {
		case fs.heartbeat <- hb:
		default:
		}
		w.WriteHeader(http.StatusNoContent)
	case r.URL.Path == "/api/agent/jobs/next":
		fs.mu.Lock()
		fs.polls = append(fs.polls, time.Now())
		var resp response
		if len(fs.queue) > 0 {
			resp, fs.queue = fs.queue[0], fs.queue[1:]
		} else {
			resp = response{status: http.StatusNoContent}
		}
		fs.mu.Unlock()
		if resp.job != nil {
			json.NewEncoder(w).Encode(resp.job)
			return
		}
		w.WriteHeader(resp.status)
	case strings.HasSuffix(r.URL.Path, "/ack"):
		id := strings.TrimSuffix(strings.TrimPrefix(r.URL.Path, "/api/agent/jobs/"), "/ack")
		var ack client.Ack
		json.NewDecoder(r.Body).Decode(&ack)
		fs.mu.Lock()
		fs.acks[id] = ack
		fs.mu.Unlock()
		fs.ackCh <- id
		w.WriteHeader(http.StatusNoContent)
	default:
		fs.t.Errorf("unexpected request %s %s", r.Method, r.URL)
		w.WriteHeader(http.StatusNotFound)
	}
}

func (fs *fakeServer) waitAck(t *testing.T, id string) client.Ack {
	t.Helper()
	deadline := time.After(5 * time.Second)
	for {
		select {
		case got := <-fs.ackCh:
			if got == id {
				fs.mu.Lock()
				defer fs.mu.Unlock()
				return fs.acks[id]
			}
		case <-deadline:
			t.Fatalf("no ack for job %s", id)
		}
	}
}

func fakePrinter(t *testing.T) (printer.Printer, <-chan []byte) {
	t.Helper()
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { ln.Close() })
	got := make(chan []byte, 16)
	go func() {
		for {
			conn, err := ln.Accept()
			if err != nil {
				return
			}
			data, _ := io.ReadAll(conn)
			conn.Close()
			got <- data
		}
	}()
	p, err := printer.Open("tcp://" + ln.Addr().String())
	if err != nil {
		t.Fatal(err)
	}
	return p, got
}

func start(t *testing.T, cfg Config) {
	t.Helper()
	cfg.Logger = slog.New(slog.NewTextHandler(io.Discard, nil))
	cfg.Version = "test"
	cfg.Hostname = "host"
	cfg.PollWait = time.Second
	if cfg.IdleDelay == 0 {
		cfg.IdleDelay = 10 * time.Millisecond
	}
	if cfg.Backoff.Min == 0 {
		cfg.Backoff = Backoff{Min: 10 * time.Millisecond, Max: 50 * time.Millisecond}
	}
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	go func() {
		New(cfg).Run(ctx)
		close(done)
	}()
	t.Cleanup(func() {
		cancel()
		select {
		case <-done:
		case <-time.After(5 * time.Second):
			t.Error("agent did not stop")
		}
	})
}

func job(id string, data []byte) *client.Job {
	return &client.Job{ID: id, Format: "zpl", Copies: 1, PayloadBase64: base64.StdEncoding.EncodeToString(data)}
}

func TestJobPrintedAndAcked(t *testing.T) {
	p, got := fakePrinter(t)
	fs, c := newFakeServer(t, response{status: http.StatusNoContent}, response{job: job("j1", gs1ZPL)})
	start(t, Config{Server: c, Printer: p})

	select {
	case data := <-got:
		if !bytes.Equal(data, gs1ZPL) {
			t.Fatalf("printer got %q, want %q", data, gs1ZPL)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("printer got nothing")
	}
	if ack := fs.waitAck(t, "j1"); ack.Status != StatusPrinted || ack.Error != "" {
		t.Fatalf("ack %+v", ack)
	}
	select {
	case hb := <-fs.heartbeat:
		if hb.Version != "test" || hb.Hostname != "host" || hb.Printer != p.URI() {
			t.Fatalf("heartbeat %+v", hb)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("no heartbeat at start")
	}
}

func TestNoContentKeepsPolling(t *testing.T) {
	p, _ := fakePrinter(t)
	fs, c := newFakeServer(t)
	start(t, Config{Server: c, Printer: p, IdleDelay: 20 * time.Millisecond})
	time.Sleep(300 * time.Millisecond)
	fs.mu.Lock()
	n, acks := len(fs.polls), len(fs.acks)
	fs.mu.Unlock()
	if n < 3 || n > 30 {
		t.Fatalf("expected steady polling paced by idle delay, got %d polls", n)
	}
	if acks != 0 {
		t.Fatalf("unexpected acks: %d", acks)
	}
}

func TestPrinterFailureAcksFailed(t *testing.T) {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	addr := ln.Addr().String()
	ln.Close()
	p, err := printer.Open("tcp://" + addr)
	if err != nil {
		t.Fatal(err)
	}
	fs, c := newFakeServer(t, response{job: job("j2", gs1ZPL)})
	start(t, Config{Server: c, Printer: p})
	ack := fs.waitAck(t, "j2")
	if ack.Status != StatusFailed || !strings.Contains(ack.Error, addr) {
		t.Fatalf("ack %+v", ack)
	}
}

func TestInvalidZPLAcksFailed(t *testing.T) {
	p, got := fakePrinter(t)
	fs, c := newFakeServer(t, response{job: job("j3", []byte("hello"))})
	start(t, Config{Server: c, Printer: p})
	if ack := fs.waitAck(t, "j3"); ack.Status != StatusFailed || !strings.Contains(ack.Error, "^XA") {
		t.Fatalf("ack %+v", ack)
	}
	select {
	case data := <-got:
		t.Fatalf("invalid zpl reached printer: %q", data)
	default:
	}
}

func TestUnauthorizedBacksOff(t *testing.T) {
	p, _ := fakePrinter(t)
	fs, c := newFakeServer(t,
		response{status: http.StatusUnauthorized},
		response{status: http.StatusUnauthorized},
		response{status: http.StatusUnauthorized},
		response{job: job("j4", gs1ZPL)},
	)
	start(t, Config{Server: c, Printer: p, Backoff: Backoff{Min: 40 * time.Millisecond, Max: 100 * time.Millisecond}})
	fs.waitAck(t, "j4")

	fs.mu.Lock()
	polls := append([]time.Time(nil), fs.polls...)
	fs.mu.Unlock()
	want := []time.Duration{40 * time.Millisecond, 80 * time.Millisecond, 100 * time.Millisecond}
	for i, min := range want {
		if gap := polls[i+1].Sub(polls[i]); gap < min {
			t.Errorf("gap after 401 #%d = %v, want >= %v", i+1, gap, min)
		}
	}
}

func TestOverrideDisallowedByDefault(t *testing.T) {
	p, got := fakePrinter(t)
	dir := t.TempDir()
	j := job("j5", gs1ZPL)
	j.Printer = "file://" + filepath.ToSlash(dir)
	fs, c := newFakeServer(t, response{job: j})
	start(t, Config{Server: c, Printer: p})

	ack := fs.waitAck(t, "j5")
	if ack.Status != StatusFailed || !strings.Contains(ack.Error, "override") {
		t.Fatalf("ack %+v", ack)
	}
	if entries, _ := os.ReadDir(dir); len(entries) != 0 {
		t.Fatalf("override printer was used: %v", entries)
	}
	select {
	case data := <-got:
		t.Fatalf("default printer was used: %q", data)
	default:
	}
}

func TestOverrideAllowedWritesFile(t *testing.T) {
	p, _ := fakePrinter(t)
	dir := t.TempDir()
	j := job("j6", gs1ZPL)
	j.Copies = 2
	j.Printer = "file://" + filepath.ToSlash(dir)
	fs, c := newFakeServer(t, response{job: j})
	start(t, Config{Server: c, Printer: p, AllowOverride: true})

	if ack := fs.waitAck(t, "j6"); ack.Status != StatusPrinted {
		t.Fatalf("ack %+v", ack)
	}
	for _, name := range []string{"j6-1.zpl", "j6-2.zpl"} {
		data, err := os.ReadFile(filepath.Join(dir, name))
		if err != nil {
			t.Fatal(err)
		}
		if !bytes.Equal(data, gs1ZPL) {
			t.Fatalf("%s differs: %q", name, data)
		}
	}
}

func TestBadBase64AcksFailed(t *testing.T) {
	p, _ := fakePrinter(t)
	j := job("j7", nil)
	j.PayloadBase64 = "@@@"
	fs, c := newFakeServer(t, response{job: j})
	start(t, Config{Server: c, Printer: p})
	if ack := fs.waitAck(t, "j7"); ack.Status != StatusFailed || !strings.Contains(ack.Error, "base64") {
		t.Fatalf("ack %+v", ack)
	}
}

func TestBackoffCaps(t *testing.T) {
	b := Backoff{Min: time.Second, Max: 5 * time.Second}
	var got []time.Duration
	for i := 0; i < 5; i++ {
		got = append(got, b.Next())
	}
	want := []time.Duration{time.Second, 2 * time.Second, 4 * time.Second, 5 * time.Second, 5 * time.Second}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("backoff %v, want %v", got, want)
		}
	}
	b.Reset()
	if b.Next() != time.Second {
		t.Fatal("reset did not restart backoff")
	}
}
