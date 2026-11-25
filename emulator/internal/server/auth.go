package server

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"errors"
	"net/http"
	"strings"
	"time"

	"markirovka/emulator/internal/gs1"
	"markirovka/emulator/internal/sign"
	"markirovka/emulator/internal/store"
)

const (
	authKeyTTL   = 10 * time.Minute
	authKeyChars = "ABCDEFGHIJKLMNOPQRSTUVWXYZ"
	authFailMsg  = "Ошибка авторизации или компания не зарегистрирована в ИС МПТ"
)

var (
	errNoToken      = errors.New("no token")
	errBadToken     = errors.New("bad token")
	errExpiredToken = errors.New("expired token")
)

type authKey struct {
	data    string
	created time.Time
}

type claims struct {
	PID string `json:"pid"`
	BIN string `json:"bin"`
	IAT int64  `json:"iat"`
	EXP int64  `json:"exp"`
	JTI string `json:"jti"`
}

func (s *Server) handleAuthKey(w http.ResponseWriter, r *http.Request) {
	id := store.NewUUID()
	data := gs1.RandomString(authKeyChars, 30)
	now := s.now()
	s.keysMu.Lock()
	for k, v := range s.keys {
		if now.Sub(v.created) > authKeyTTL {
			delete(s.keys, k)
		}
	}
	s.keys[id] = authKey{data: data, created: now}
	s.keysMu.Unlock()
	writeJSON(w, http.StatusOK, map[string]string{"uuid": id, "data": data})
}

func (s *Server) handleSignIn(w http.ResponseWriter, r *http.Request) {
	var req struct {
		UUID string `json:"uuid"`
		Data string `json:"data"`
	}
	if !decodeJSON(w, r, &req) {
		return
	}
	if req.UUID == "" {
		writeError(w, http.StatusBadRequest, "Отсутствует обязательный параметр: uuid")
		return
	}
	if req.Data == "" {
		writeError(w, http.StatusBadRequest, "Отсутствует обязательный параметр: data")
		return
	}
	s.keysMu.Lock()
	key, ok := s.keys[req.UUID]
	delete(s.keys, req.UUID)
	s.keysMu.Unlock()
	if !ok || s.now().Sub(key.created) > authKeyTTL {
		authFail(w, "Идентификатор uuid не найден или устарел, запросите новый через /auth/key")
		return
	}
	res, err := sign.Verify(req.Data, s.signOpts())
	if err != nil {
		authFail(w, err.Error())
		return
	}
	bin := res.BIN
	if res.Emulated {
		if string(res.Data) != key.data {
			authFail(w, "Подписанные данные не совпадают с data из /auth/key")
			return
		}
	} else {
		bin = s.store.SeedBIN()
	}
	if _, ok := s.store.Participant(bin); !ok {
		authFail(w, "Участник с ИИН/БИН "+bin+" не зарегистрирован в ИС МПТ")
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"token": s.issueToken(bin)})
}

func authFail(w http.ResponseWriter, description string) {
	writeJSON(w, http.StatusUnauthorized, map[string]string{
		"code":          "401",
		"error_message": authFailMsg,
		"description":   description,
	})
}

func (s *Server) signOpts() sign.Options {
	return sign.Options{AcceptEmulated: s.cfg.AcceptEmulatedSignature, AcceptAnyCMS: s.cfg.AcceptAnyCMS}
}

var b64url = base64.RawURLEncoding

func (s *Server) issueToken(bin string) string {
	now := s.now()
	header := b64url.EncodeToString([]byte(`{"alg":"HS256","typ":"JWT"}`))
	payload, _ := json.Marshal(claims{
		PID: bin,
		BIN: bin,
		IAT: now.Unix(),
		EXP: now.Add(s.cfg.TokenTTL).Unix(),
		JTI: store.NewUUID(),
	})
	unsigned := header + "." + b64url.EncodeToString(payload)
	return unsigned + "." + b64url.EncodeToString(s.tokenMAC(unsigned))
}

func (s *Server) tokenMAC(unsigned string) []byte {
	m := hmac.New(sha256.New, s.store.Secret())
	m.Write([]byte(unsigned))
	return m.Sum(nil)
}

func bearer(r *http.Request) string {
	h := strings.TrimSpace(r.Header.Get("Authorization"))
	if len(h) < 6 || !strings.EqualFold(h[:6], "bearer") {
		return ""
	}
	return strings.TrimSpace(strings.TrimPrefix(h[6:], ":"))
}

func (s *Server) parseToken(token string) (claims, error) {
	if token == "" {
		return claims{}, errNoToken
	}
	parts := strings.Split(token, ".")
	if len(parts) != 3 {
		return claims{}, errBadToken
	}
	sig, err := b64url.DecodeString(parts[2])
	if err != nil || !hmac.Equal(sig, s.tokenMAC(parts[0]+"."+parts[1])) {
		return claims{}, errBadToken
	}
	raw, err := b64url.DecodeString(parts[1])
	if err != nil {
		return claims{}, errBadToken
	}
	var c claims
	if err := json.Unmarshal(raw, &c); err != nil || c.BIN == "" {
		return claims{}, errBadToken
	}
	if s.now().Unix() >= c.EXP {
		return claims{}, errExpiredToken
	}
	return c, nil
}

type authedHandler func(w http.ResponseWriter, r *http.Request, bin string)

func (s *Server) authed(h authedHandler) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		token := bearer(r)
		c, err := s.parseToken(token)
		if err != nil {
			writeUnauthorized(w, err, token)
			return
		}
		if _, ok := s.store.Participant(c.BIN); !ok {
			writeUnauthorized(w, errBadToken, token)
			return
		}
		h(w, r, c.BIN)
	}
}

func writeUnauthorized(w http.ResponseWriter, err error, token string) {
	w.Header().Set("Content-Type", "application/xml;charset=UTF-8")
	w.WriteHeader(http.StatusUnauthorized)
	switch {
	case errors.Is(err, errNoToken):
		w.Write([]byte("<UnauthorizedException><error>unauthorized</error><error_description>Full authentication is required to access this resource</error_description></UnauthorizedException>"))
	case errors.Is(err, errExpiredToken):
		w.Write([]byte("<InvalidTokenException><error>invalid_token</error><error_description>Access token expired: " + xmlEscape(token) + "</error_description></InvalidTokenException>"))
	default:
		w.Write([]byte("<InvalidTokenException><error>invalid_token</error><error_description>Cannot convert access token to JSON</error_description></InvalidTokenException>"))
	}
}

func xmlEscape(s string) string {
	return strings.NewReplacer("&", "&amp;", "<", "&lt;", ">", "&gt;", `"`, "&quot;", "'", "&apos;").Replace(s)
}
