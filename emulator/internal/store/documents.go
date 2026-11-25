package store

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"sort"
	"strings"

	"markirovka/emulator/internal/gs1"
)

type docBody struct {
	FileID          string `json:"fileId"`
	Version         string `json:"version"`
	SendingDateTime string `json:"sendingDateTime"`
	Document        *struct {
		MainInfo struct {
			Reason          json.RawMessage `json:"reason"`
			ParticipantInfo struct {
				ParticipantName               string `json:"participantName"`
				ParticipantIdentificationCode string `json:"participantIdentificationCode"`
			} `json:"participantInfo"`
		} `json:"mainInfo"`
		ProductList []struct {
			ICode string `json:"iCode"`
		} `json:"productList"`
	} `json:"document"`
}

type NewDocument struct {
	Type    string
	Content string
	Body    []byte
}

func (s *Store) CreateDocument(bin string, nd NewDocument) (string, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, ok := DocumentTypes[nd.Type]; !ok {
		return "", newError(http.StatusBadRequest, "Неизвестный тип документа: "+nd.Type)
	}
	var body docBody
	dec := json.NewDecoder(bytes.NewReader(nd.Body))
	if err := dec.Decode(&body); err != nil {
		return "", newError(http.StatusBadRequest, "Невалидный формат документа: "+err.Error())
	}
	if body.Document == nil || len(body.Document.ProductList) == 0 {
		return "", newError(http.StatusBadRequest, "Отсутствует обязательный параметр: document.productList")
	}
	if pid := body.Document.MainInfo.ParticipantInfo.ParticipantIdentificationCode; pid != "" && pid != bin {
		return "", newError(http.StatusBadRequest, "ИИН/БИН участника в документе не совпадает с токеном")
	}
	if nd.Type == DocWithdraw && !validReason(body.Document.MainInfo.Reason) {
		return "", newError(http.StatusBadRequest, "Отсутствует обязательный параметр: document.mainInfo.reason (1..8)")
	}
	codes := make([]string, 0, len(body.Document.ProductList))
	for _, p := range body.Document.ProductList {
		codes = append(codes, p.ICode)
	}
	id := strings.TrimSpace(body.FileID)
	if id == "" {
		id = NewUUID()
	}
	if d, ok := s.st.Documents[id]; ok {
		if d.SenderBIN == bin && d.Content == nd.Content {
			return id, nil
		}
		return "", newError(http.StatusConflict, "Документ с fileId "+id+" уже зарегистрирован")
	}
	now := s.realNow()
	s.st.DocSeq++
	d := &Document{
		ID:         id,
		Number:     fmt.Sprintf("%08d", s.st.DocSeq),
		Type:       nd.Type,
		Status:     DocInProgress,
		SenderBIN:  bin,
		DocDate:    s.emuNow(),
		ReceivedAt: now,
		ProcessAt:  now.Add(s.cfg.DocDelay),
		Content:    nd.Content,
		Body:       json.RawMessage(compactJSON(nd.Body)),
		Codes:      codes,
	}
	if p, ok := s.st.Participants[bin]; ok {
		d.SenderName = p.Name
	}
	s.st.Documents[id] = d
	if s.cfg.DocDelay <= 0 {
		s.process(d)
	}
	return id, s.save()
}

func validReason(raw json.RawMessage) bool {
	var v any
	if json.Unmarshal(raw, &v) != nil {
		return false
	}
	r := strings.TrimSpace(fmt.Sprint(v))
	return len(r) == 1 && r[0] >= '1' && r[0] <= '8'
}

func compactJSON(b []byte) []byte {
	var buf bytes.Buffer
	if err := json.Compact(&buf, b); err != nil {
		return []byte("null")
	}
	return buf.Bytes()
}

func (s *Store) process(d *Document) {
	t := DocumentTypes[d.Type]
	emu := s.emuNow()
	seen := map[string]bool{}
	targets := make([]*Code, 0, len(d.Codes))
	var errs []CisError
	fail := func(cis, code, msg string) {
		errs = append(errs, CisError{Cis: cis, ErrorCode: code, ErrorMessage: msg})
	}
	for _, raw := range d.Codes {
		parsed, err := gs1.Parse(raw)
		if err != nil {
			code := "CIS_FORMAT"
			if errors.Is(err, gs1.ErrGSLost) {
				code = "GS_LOST"
			}
			fail(raw, code, err.Error())
			continue
		}
		ki := parsed.KI()
		cis := parsed.CIS()
		if seen[ki] {
			fail(cis, "CIS_DUPLICATE", "КМ указан в документе повторно")
			continue
		}
		seen[ki] = true
		c, ok := s.st.Codes[ki]
		switch {
		case !ok:
			fail(cis, "CIS_NOT_FOUND", "КМ не найден в ИС МПТ")
		case c.OwnerBIN != d.SenderBIN:
			fail(cis, "CIS_NOT_OWNED", "КМ не принадлежит участнику "+d.SenderBIN)
		case parsed.HasCrypto() && (parsed.Key != c.Key || parsed.Crypto != c.Crypto):
			fail(cis, "CRYPTO_MISMATCH", "Код проверки (91/92) не совпадает с эмитированным")
		case c.Annulled:
			fail(cis, "CIS_ANNULLED", fmt.Sprintf("КМ аннулирован: не нанесён в течение %d дней с эмиссии", s.cfg.AnnulDays))
		case c.Status != t.From:
			fail(cis, "CIS_WRONG_STATUS", fmt.Sprintf("Недопустимый статус КМ %s, ожидается %s", c.Status, t.From))
		default:
			targets = append(targets, c)
		}
	}
	now := s.realNow()
	d.ProcessedAt = &now
	if len(errs) > 0 {
		d.Status = DocCheckedNotOK
		d.CisErrors = errs
		d.ErrorMessage = fmt.Sprintf("Документ не прошёл проверку: ошибок по КМ %d из %d", len(errs), len(d.Codes))
		return
	}
	for _, c := range targets {
		at := emu
		c.Status = t.To
		switch t.To {
		case StatusApplied:
			c.AppliedAt = &at
		case StatusIntroduced:
			c.IntroducedAt = &at
		case StatusWithdrawn:
			c.WithdrawnAt = &at
		}
	}
	d.Status = DocCheckedOK
}

func (s *Store) Document(bin, id string) (Document, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.advance() {
		_ = s.save()
	}
	d, ok := s.st.Documents[id]
	if !ok || d.SenderBIN != bin {
		return Document{}, newError(http.StatusNotFound, "Документ с указанным идентификатором не найден")
	}
	return *d, nil
}

func (s *Store) Documents(bin string, limit int) []Document {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.advance() {
		_ = s.save()
	}
	return s.documents(bin, limit)
}

func (s *Store) documents(bin string, limit int) []Document {
	out := make([]Document, 0)
	for _, d := range s.st.Documents {
		if bin == "" || d.SenderBIN == bin {
			out = append(out, *d)
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Number > out[j].Number })
	if limit > 0 && len(out) > limit {
		out = out[:limit]
	}
	return out
}

func (d Document) Errors() []string {
	out := make([]string, 0, len(d.CisErrors))
	for _, e := range d.CisErrors {
		out = append(out, e.Cis+": "+e.ErrorMessage)
	}
	return out
}
