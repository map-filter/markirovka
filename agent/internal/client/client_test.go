package client

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func newClient(t *testing.T, h http.HandlerFunc) *Client {
	t.Helper()
	srv := httptest.NewTLSServer(h)
	t.Cleanup(srv.Close)
	c, err := New(Options{BaseURL: srv.URL + "/", Token: "secret", InsecureTLS: true, UserAgent: "test"})
	if err != nil {
		t.Fatal(err)
	}
	return c
}

func TestNextJob(t *testing.T) {
	c := newClient(t, func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet || r.URL.Path != "/api/agent/jobs/next" || r.URL.Query().Get("wait") != "7" {
			t.Errorf("unexpected request %s %s", r.Method, r.URL)
		}
		if r.Header.Get("Authorization") != "Bearer secret" {
			t.Errorf("auth header %q", r.Header.Get("Authorization"))
		}
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"id":"a1","format":"zpl","copies":2,"printer":"","payload_base64":"XlhB"}`))
	})
	job, err := c.NextJob(context.Background(), 7*time.Second)
	if err != nil {
		t.Fatal(err)
	}
	want := Job{ID: "a1", Format: "zpl", Copies: 2, PayloadBase64: "XlhB"}
	if *job != want {
		t.Fatalf("job %+v", *job)
	}
}

func TestNextJobNoContent(t *testing.T) {
	c := newClient(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	})
	job, err := c.NextJob(context.Background(), time.Second)
	if err != nil || job != nil {
		t.Fatalf("job=%v err=%v", job, err)
	}
}

func TestNextJobUnauthorized(t *testing.T) {
	c := newClient(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
	})
	if _, err := c.NextJob(context.Background(), time.Second); !errors.Is(err, ErrUnauthorized) {
		t.Fatalf("err %v", err)
	}
}

func TestNextJobServerError(t *testing.T) {
	c := newClient(t, func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "boom", http.StatusBadGateway)
	})
	if _, err := c.NextJob(context.Background(), time.Second); err == nil || errors.Is(err, ErrUnauthorized) {
		t.Fatalf("err %v", err)
	}
}

func TestAckAndHeartbeat(t *testing.T) {
	var ackPath string
	var ack Ack
	var hb Heartbeat
	c := newClient(t, func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost || r.Header.Get("Content-Type") != "application/json" {
			t.Errorf("unexpected %s %q", r.Method, r.Header.Get("Content-Type"))
		}
		switch r.URL.Path {
		case "/api/agent/heartbeat":
			json.NewDecoder(r.Body).Decode(&hb)
		default:
			ackPath = r.URL.EscapedPath()
			json.NewDecoder(r.Body).Decode(&ack)
		}
		w.WriteHeader(http.StatusNoContent)
	})
	if err := c.Ack(context.Background(), "id 1/2", Ack{Status: "failed", Error: "jam"}); err != nil {
		t.Fatal(err)
	}
	if ackPath != "/api/agent/jobs/id%201%2F2/ack" || ack != (Ack{Status: "failed", Error: "jam"}) {
		t.Fatalf("ack %q %+v", ackPath, ack)
	}
	if err := c.Heartbeat(context.Background(), Heartbeat{Version: "v", Hostname: "h", Printer: "tcp://p:9100"}); err != nil {
		t.Fatal(err)
	}
	if hb != (Heartbeat{Version: "v", Hostname: "h", Printer: "tcp://p:9100"}) {
		t.Fatalf("heartbeat %+v", hb)
	}
}

func TestNewValidates(t *testing.T) {
	for _, o := range []Options{
		{BaseURL: "ftp://x", Token: "t"},
		{BaseURL: "https://", Token: "t"},
		{BaseURL: "https://x"},
	} {
		if _, err := New(o); err == nil {
			t.Errorf("New(%+v) should fail", o)
		}
	}
}
