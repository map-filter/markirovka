package server

import (
	"embed"
	"encoding/json"
	"fmt"
	"html/template"
	"io"
	"log"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"markirovka/emulator/internal/store"
)

//go:embed templates/console.html
var templatesFS embed.FS

var almaty = time.FixedZone("UTC+5", 5*3600)

var consoleTmpl = template.Must(template.New("console.html").Funcs(template.FuncMap{
	"t": func(v time.Time) string {
		if v.IsZero() {
			return ""
		}
		return v.In(almaty).Format("02.01.2006 15:04:05")
	},
	"short": func(s string) string {
		if len(s) > 8 {
			return s[:8]
		}
		return s
	},
	"statusTitle": func(s string) string { return store.StatusTitles[s] },
	"count":       func(m map[string]int, k string) int { return m[k] },
	"offset": func(sec int64) string {
		if sec == 0 {
			return "нет"
		}
		d := time.Duration(sec) * time.Second
		days := int(d.Hours()) / 24
		rest := d - time.Duration(days)*24*time.Hour
		if rest == 0 {
			return fmt.Sprintf("+%d дн.", days)
		}
		return fmt.Sprintf("+%d дн. %s", days, rest)
	},
	"docClass": func(s string) string {
		switch s {
		case store.DocCheckedOK, store.OrderSuccess:
			return "ok"
		case store.DocCheckedNotOK:
			return "bad"
		}
		return "wait"
	},
	"httpClass": func(code int) string {
		switch {
		case code >= 500:
			return "bad"
		case code >= 400:
			return "warn"
		}
		return "ok"
	},
}).ParseFS(templatesFS, "templates/console.html"))

type consoleData struct {
	store.Snapshot
	Log      []LogEntry
	Statuses []string
}

func (s *Server) handleConsole(w http.ResponseWriter, r *http.Request) {
	data := consoleData{
		Snapshot: s.store.Snapshot(),
		Log:      s.Log(),
		Statuses: []string{store.StatusEmitted, store.StatusApplied, store.StatusIntroduced, store.StatusWithdrawn, store.StatusWrittenOff},
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	if err := consoleTmpl.Execute(w, data); err != nil {
		log.Printf("console: %v", err)
	}
}

func (s *Server) handleState(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, map[string]any{"state": s.store.Snapshot(), "log": s.Log()})
}

func wantsHTML(r *http.Request) bool {
	return strings.Contains(r.Header.Get("Accept"), "text/html")
}

func (s *Server) handleReset(w http.ResponseWriter, r *http.Request) {
	if err := s.store.Reset(); err != nil {
		writeStoreError(w, err)
		return
	}
	s.keysMu.Lock()
	s.keys = map[string]authKey{}
	s.keysMu.Unlock()
	s.logMu.Lock()
	s.log = nil
	s.logMu.Unlock()
	s.done(w, r)
}

func (s *Server) handleTime(w http.ResponseWriter, r *http.Request) {
	var req struct {
		AddDays int  `json:"addDays"`
		Reset   bool `json:"reset"`
	}
	raw, err := io.ReadAll(r.Body)
	if err != nil {
		writeError(w, http.StatusBadRequest, "Ошибка чтения тела запроса")
		return
	}
	if trimmed := strings.TrimSpace(string(raw)); strings.HasPrefix(trimmed, "{") {
		if err := json.Unmarshal(raw, &req); err != nil {
			writeError(w, http.StatusBadRequest, "JSON parse error: "+err.Error())
			return
		}
	} else {
		form, err := url.ParseQuery(trimmed)
		if err != nil {
			writeError(w, http.StatusBadRequest, "Ошибка в параметрах запроса")
			return
		}
		if form.Get("action") == "reset" {
			req.Reset = true
		} else if req.AddDays, err = strconv.Atoi(form.Get("days")); err != nil {
			writeError(w, http.StatusBadRequest, "Параметр days требует значение типа Number")
			return
		}
	}
	switch {
	case req.Reset:
		err = s.store.ResetOffset()
	case req.AddDays > 0 && req.AddDays <= 3650:
		err = s.store.ShiftDays(req.AddDays)
	default:
		writeError(w, http.StatusBadRequest, "Параметр addDays требует значение от 1 до 3650")
		return
	}
	if err != nil {
		writeStoreError(w, err)
		return
	}
	s.done(w, r)
}

func (s *Server) done(w http.ResponseWriter, r *http.Request) {
	if wantsHTML(r) {
		http.Redirect(w, r, "/emu/", http.StatusSeeOther)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"ok":        true,
		"emuNow":    s.store.EmuNow(),
		"offsetSec": int64(s.store.Offset() / time.Second),
	})
}

func (s *Server) handleAddCard(w http.ResponseWriter, r *http.Request) {
	data, err := io.ReadAll(r.Body)
	if err != nil {
		writeError(w, http.StatusBadRequest, "Ошибка чтения тела запроса")
		return
	}
	var p store.Product
	if err := json.Unmarshal(data, &p); err != nil {
		writeError(w, http.StatusBadRequest, "JSON parse error: "+err.Error())
		return
	}
	var extra struct {
		ParticipantBIN string `json:"participantBin"`
	}
	_ = json.Unmarshal(data, &extra)
	if extra.ParticipantBIN != "" {
		p.OwnerBIN = extra.ParticipantBIN
	}
	created, err := s.store.AddProduct(p)
	if err != nil {
		writeStoreError(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, productInfo(created))
}
