package server

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"log"
	"net/http"
	"strings"
	"sync"
	"time"

	"markirovka/emulator/internal/store"
)

const (
	BasePath   = "/api/v3/true-api"
	logSize    = 100
	maxBodyLen = 30 << 20
)

type Config struct {
	AcceptEmulatedSignature bool
	AcceptAnyCMS            bool
	TokenTTL                time.Duration
}

type Server struct {
	cfg   Config
	store *store.Store
	now   func() time.Time
	mux   *http.ServeMux

	keysMu sync.Mutex
	keys   map[string]authKey

	logMu sync.Mutex
	log   []LogEntry
}

type LogEntry struct {
	Time     time.Time `json:"time"`
	Method   string    `json:"method"`
	Path     string    `json:"path"`
	Status   int       `json:"status"`
	Duration int64     `json:"durationMs"`
}

func New(cfg Config, st *store.Store, now func() time.Time) *Server {
	if now == nil {
		now = time.Now
	}
	if cfg.TokenTTL <= 0 {
		cfg.TokenTTL = 10 * time.Hour
	}
	s := &Server{cfg: cfg, store: st, now: now, mux: http.NewServeMux(), keys: map[string]authKey{}}
	s.routes()
	return s
}

func (s *Server) routes() {
	b := BasePath
	m := s.mux
	m.HandleFunc("GET "+b+"/auth/key", s.handleAuthKey)
	m.HandleFunc("POST "+b+"/auth/simpleSignIn", s.handleSignIn)
	m.HandleFunc("GET "+b+"/product/gtin", s.authed(s.handleProductGTIN))
	m.HandleFunc("GET "+b+"/products/info", s.authed(s.handleProductsInfo))
	m.HandleFunc("POST "+b+"/cises/my", s.authed(s.handleCisesMy))
	m.HandleFunc("GET "+b+"/cises/orders/{uuid}/status", s.authed(s.handleOrderStatus))
	m.HandleFunc("GET "+b+"/cises/orders/{uuid}/result", s.authed(s.handleOrderResult))
	m.HandleFunc("POST "+b+"/cises/info", s.authed(s.handleCisesInfo))
	m.HandleFunc("POST "+b+"/documents/create", s.authed(s.handleDocumentCreate))
	m.HandleFunc("POST "+b+"/documents/writeOff", s.authed(s.handleWriteOff))
	m.HandleFunc("GET "+b+"/documents/{a}/{b}", s.authed(s.handleDocumentInfo))
	m.HandleFunc("GET "+b+"/doc/listV2", s.authed(s.handleDocList))
	for _, p := range []string{b + "/edo-api/outgoing-documents/formal", "/edo-api/outgoing-documents/formal"} {
		m.HandleFunc("POST "+p, s.authed(s.handleFormal))
	}
	m.HandleFunc(b+"/", notFound)
	m.HandleFunc("/edo-api/", notFound)

	m.HandleFunc("GET /healthz", func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
	})
	m.HandleFunc("GET /emu", func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, "/emu/", http.StatusMovedPermanently)
	})
	m.HandleFunc("GET /emu/{$}", s.handleConsole)
	m.HandleFunc("GET /emu/api/state", s.handleState)
	m.HandleFunc("POST /emu/reset", s.handleReset)
	m.HandleFunc("POST /emu/api/reset", s.handleReset)
	m.HandleFunc("POST /emu/api/time", s.handleTime)
	m.HandleFunc("POST /emu/api/nkt/cards", s.handleAddCard)
	m.HandleFunc("GET /{$}", func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, "/emu/", http.StatusFound)
	})
}

func (s *Server) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	start := s.now()
	r.Body = http.MaxBytesReader(w, r.Body, maxBodyLen)
	rec := &statusRecorder{ResponseWriter: w, status: http.StatusOK}
	s.mux.ServeHTTP(rec, r)
	if strings.HasPrefix(r.URL.Path, "/api/") || strings.HasPrefix(r.URL.Path, "/edo-api/") {
		path := r.URL.Path
		if r.URL.RawQuery != "" {
			path += "?" + r.URL.RawQuery
		}
		if len(path) > 160 {
			path = path[:160] + "..."
		}
		s.addLog(LogEntry{Time: start.UTC(), Method: r.Method, Path: path, Status: rec.status, Duration: s.now().Sub(start).Milliseconds()})
	}
}

func (s *Server) Run(ctx context.Context, interval time.Duration) {
	t := time.NewTicker(interval)
	defer t.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-t.C:
			if err := s.store.Advance(); err != nil {
				log.Printf("advance: %v", err)
			}
		}
	}
}

func (s *Server) addLog(e LogEntry) {
	s.logMu.Lock()
	defer s.logMu.Unlock()
	s.log = append(s.log, e)
	if len(s.log) > logSize {
		s.log = s.log[len(s.log)-logSize:]
	}
}

func (s *Server) Log() []LogEntry {
	s.logMu.Lock()
	defer s.logMu.Unlock()
	out := make([]LogEntry, len(s.log))
	for i, e := range s.log {
		out[len(s.log)-1-i] = e
	}
	return out
}

type statusRecorder struct {
	http.ResponseWriter
	status int
}

func (r *statusRecorder) WriteHeader(code int) {
	r.status = code
	r.ResponseWriter.WriteHeader(code)
}

func notFound(w http.ResponseWriter, r *http.Request) {
	writeError(w, http.StatusNotFound, "Метод с указанным URL не найден")
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json;charset=UTF-8")
	w.WriteHeader(status)
	enc := json.NewEncoder(w)
	enc.SetEscapeHTML(false)
	_ = enc.Encode(v)
}

func writeError(w http.ResponseWriter, status int, msg string) {
	writeJSON(w, status, map[string]string{"error_message": msg})
}

func writeStoreError(w http.ResponseWriter, err error) {
	var se *store.Error
	if errors.As(err, &se) {
		writeError(w, se.Status, se.Message)
		return
	}
	log.Printf("internal error: %v", err)
	writeError(w, http.StatusInternalServerError, "Ошибка при выполнении запроса")
}

func decodeJSON(w http.ResponseWriter, r *http.Request, v any) bool {
	data, err := io.ReadAll(r.Body)
	if err != nil {
		var mbe *http.MaxBytesError
		if errors.As(err, &mbe) {
			writeError(w, http.StatusRequestEntityTooLarge, "Слишком большой запрос")
			return false
		}
		writeError(w, http.StatusBadRequest, "Ошибка чтения тела запроса")
		return false
	}
	if len(strings.TrimSpace(string(data))) == 0 {
		writeError(w, http.StatusBadRequest, "Required request body is missing")
		return false
	}
	if err := json.Unmarshal(data, v); err != nil {
		writeError(w, http.StatusBadRequest, "JSON parse error: "+err.Error())
		return false
	}
	return true
}
