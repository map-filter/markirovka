package store

import (
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"sync"
	"time"

	"markirovka/emulator/internal/gs1"
)

type Config struct {
	SeedFile       string
	StateFile      string
	OrderDelay     time.Duration
	DocDelay       time.Duration
	AnnulDays      int
	AnyParticipant bool
}

type Store struct {
	mu    sync.Mutex
	cfg   Config
	now   func() time.Time
	st    *State
	dirty bool
}

func New(cfg Config, now func() time.Time) (*Store, error) {
	if now == nil {
		now = time.Now
	}
	s := &Store{cfg: cfg, now: now}
	if cfg.StateFile != "" {
		data, err := os.ReadFile(cfg.StateFile)
		switch {
		case err == nil:
			var st State
			if err := json.Unmarshal(data, &st); err != nil {
				return nil, fmt.Errorf("state file %s: %w", cfg.StateFile, err)
			}
			st.ensureMaps()
			s.st = &st
			return s, nil
		case !errors.Is(err, os.ErrNotExist):
			return nil, err
		}
	}
	st, err := s.fromSeed("")
	if err != nil {
		return nil, err
	}
	s.st = st
	return s, s.save()
}

func (st *State) ensureMaps() {
	if st.Participants == nil {
		st.Participants = map[string]*Participant{}
	}
	if st.Products == nil {
		st.Products = map[string]*Product{}
	}
	if st.Codes == nil {
		st.Codes = map[string]*Code{}
	}
	if st.Orders == nil {
		st.Orders = map[string]*Order{}
	}
	if st.Documents == nil {
		st.Documents = map[string]*Document{}
	}
}

func (s *Store) fromSeed(secret string) (*State, error) {
	data, err := os.ReadFile(s.cfg.SeedFile)
	if err != nil {
		return nil, fmt.Errorf("seed: %w", err)
	}
	var seed Seed
	if err := json.Unmarshal(data, &seed); err != nil {
		return nil, fmt.Errorf("seed %s: %w", s.cfg.SeedFile, err)
	}
	if seed.Participant.BIN == "" {
		return nil, errors.New("seed: participant.bin is empty")
	}
	if secret == "" {
		secret = randomHex(32)
	}
	st := &State{Secret: secret, SeedBIN: seed.Participant.BIN}
	st.ensureMaps()
	p := seed.Participant
	st.Participants[p.BIN] = &p
	created := s.realNow()
	for _, pr := range seed.Products {
		if err := gs1.ValidateGTIN(pr.GTIN); err != nil {
			return nil, fmt.Errorf("seed product %s: %w", pr.GTIN, err)
		}
		pr := pr
		st.NextProductID++
		pr.ID = st.NextProductID
		pr.OwnerBIN = p.BIN
		pr.CreatedAt = created
		st.Products[pr.GTIN] = &pr
	}
	return st, nil
}

func (s *Store) SeedBIN() string {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.seedBIN()
}

func (s *Store) seedBIN() string {
	return s.st.SeedBIN
}

func (s *Store) Secret() []byte {
	s.mu.Lock()
	defer s.mu.Unlock()
	return []byte(s.st.Secret)
}

func (s *Store) Reset() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	st, err := s.fromSeed(s.st.Secret)
	if err != nil {
		return err
	}
	s.st = st
	return s.save()
}

func (s *Store) realNow() time.Time {
	return s.now().UTC().Truncate(time.Millisecond)
}

func (s *Store) emuNow() time.Time {
	return s.realNow().Add(time.Duration(s.st.OffsetSec) * time.Second)
}

func (s *Store) EmuNow() time.Time {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.emuNow()
}

func (s *Store) Offset() time.Duration {
	s.mu.Lock()
	defer s.mu.Unlock()
	return time.Duration(s.st.OffsetSec) * time.Second
}

func (s *Store) ShiftDays(days int) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.st.OffsetSec += int64(days) * 86400
	s.advance()
	return s.save()
}

func (s *Store) ResetOffset() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.st.OffsetSec = 0
	return s.save()
}

func (s *Store) Participant(bin string) (Participant, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	p, ok := s.st.Participants[bin]
	if ok {
		return *p, true
	}
	if s.cfg.AnyParticipant && bin != "" {
		np := &Participant{BIN: bin, Name: "Участник " + bin}
		s.st.Participants[bin] = np
		_ = s.save()
		return *np, true
	}
	return Participant{}, false
}

func (s *Store) Advance() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.advance() {
		return s.save()
	}
	return nil
}

func (s *Store) advance() bool {
	changed := false
	now := s.realNow()
	emu := s.emuNow()
	limit := time.Duration(s.cfg.AnnulDays) * 24 * time.Hour
	for _, c := range s.st.Codes {
		if c.Status == StatusEmitted && !emu.Before(c.EmittedAt.Add(limit)) {
			c.Status = StatusWrittenOff
			c.Annulled = true
			t := emu
			c.AnnulledAt = &t
			changed = true
		}
	}
	docs := make([]*Document, 0)
	for _, d := range s.st.Documents {
		if d.Status == DocInProgress && !now.Before(d.ProcessAt) {
			docs = append(docs, d)
		}
	}
	sort.Slice(docs, func(i, j int) bool { return docs[i].Number < docs[j].Number })
	for _, d := range docs {
		s.process(d)
		changed = true
	}
	return changed
}

func (s *Store) save() error {
	if s.cfg.StateFile == "" {
		return nil
	}
	data, err := json.Marshal(s.st)
	if err != nil {
		return err
	}
	dir := filepath.Dir(s.cfg.StateFile)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	tmp, err := os.CreateTemp(dir, ".state-*.json")
	if err != nil {
		return err
	}
	if _, err := tmp.Write(data); err != nil {
		tmp.Close()
		os.Remove(tmp.Name())
		return err
	}
	if err := tmp.Close(); err != nil {
		os.Remove(tmp.Name())
		return err
	}
	return os.Rename(tmp.Name(), s.cfg.StateFile)
}

func (s *Store) mac(parts ...string) []byte {
	m := hmac.New(sha256.New, []byte(s.st.Secret))
	for _, p := range parts {
		m.Write([]byte(p))
		m.Write([]byte{0})
	}
	return m.Sum(nil)
}

func (s *Store) cryptoTail(ki string) (string, string) {
	sum := s.mac("91", ki)
	key := make([]byte, gs1.KeyLength)
	for i := range key {
		key[i] = gs1.KeyCharset[int(sum[i])%len(gs1.KeyCharset)]
	}
	return string(key), base64.StdEncoding.EncodeToString(s.mac("92", ki))
}

func randomHex(n int) string {
	b := make([]byte, n)
	if _, err := rand.Read(b); err != nil {
		panic(err)
	}
	return hex.EncodeToString(b)
}

func NewUUID() string {
	b := make([]byte, 16)
	if _, err := rand.Read(b); err != nil {
		panic(err)
	}
	b[6] = b[6]&0x0f | 0x40
	b[8] = b[8]&0x3f | 0x80
	return fmt.Sprintf("%x-%x-%x-%x-%x", b[0:4], b[4:6], b[6:8], b[8:10], b[10:16])
}
