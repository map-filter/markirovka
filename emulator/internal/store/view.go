package store

import (
	"sort"
	"time"
)

type OrderView struct {
	UUID      string    `json:"uuid"`
	OwnerBIN  string    `json:"ownerBin"`
	Kind      string    `json:"kind"`
	GTIN      string    `json:"gtin"`
	Product   string    `json:"product"`
	Quantity  int       `json:"quantity"`
	Status    string    `json:"status"`
	CreatedAt time.Time `json:"createdAt"`
	ReadyAt   time.Time `json:"readyAt"`
}

type DocumentView struct {
	ID           string     `json:"id"`
	Number       string     `json:"number"`
	Type         string     `json:"type"`
	TypeTitle    string     `json:"typeTitle"`
	Status       string     `json:"status"`
	SenderBIN    string     `json:"senderBin"`
	Codes        int        `json:"codes"`
	ReceivedAt   time.Time  `json:"receivedAt"`
	ProcessedAt  *time.Time `json:"processedAt,omitempty"`
	ErrorMessage string     `json:"errorMessage,omitempty"`
	CisErrors    []CisError `json:"cisErrors,omitempty"`
}

type StatusCount struct {
	Status string `json:"status"`
	Title  string `json:"title"`
	Count  int    `json:"count"`
}

type ProductView struct {
	Product
	Codes map[string]int `json:"codes"`
}

type Snapshot struct {
	Now          time.Time      `json:"now"`
	EmuNow       time.Time      `json:"emuNow"`
	OffsetSec    int64          `json:"offsetSec"`
	AnnulDays    int            `json:"annulDays"`
	Participants []Participant  `json:"participants"`
	Products     []ProductView  `json:"products"`
	Orders       []OrderView    `json:"orders"`
	CodeCounts   []StatusCount  `json:"codeCounts"`
	TotalCodes   int            `json:"totalCodes"`
	Documents    []DocumentView `json:"documents"`
}

var statusOrder = []string{StatusEmitted, StatusApplied, StatusIntroduced, StatusWithdrawn, StatusWrittenOff}

func (s *Store) Snapshot() Snapshot {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.advance() {
		_ = s.save()
	}
	snap := Snapshot{
		Now:       s.realNow(),
		EmuNow:    s.emuNow(),
		OffsetSec: s.st.OffsetSec,
		AnnulDays: s.cfg.AnnulDays,
	}
	for _, p := range s.st.Participants {
		snap.Participants = append(snap.Participants, *p)
	}
	sort.Slice(snap.Participants, func(i, j int) bool { return snap.Participants[i].BIN < snap.Participants[j].BIN })

	perGTIN := map[string]map[string]int{}
	total := map[string]int{}
	for _, c := range s.st.Codes {
		total[c.Status]++
		if perGTIN[c.GTIN] == nil {
			perGTIN[c.GTIN] = map[string]int{}
		}
		perGTIN[c.GTIN][c.Status]++
		snap.TotalCodes++
	}
	for _, st := range statusOrder {
		snap.CodeCounts = append(snap.CodeCounts, StatusCount{Status: st, Title: StatusTitles[st], Count: total[st]})
	}
	for _, p := range s.productsOf("") {
		counts := perGTIN[p.GTIN]
		if counts == nil {
			counts = map[string]int{}
		}
		snap.Products = append(snap.Products, ProductView{Product: p, Codes: counts})
	}
	for _, o := range s.st.Orders {
		v := OrderView{UUID: o.UUID, OwnerBIN: o.OwnerBIN, Kind: o.Kind, GTIN: o.GTIN, Quantity: o.Quantity,
			Status: s.orderStatus(o), CreatedAt: o.CreatedAt, ReadyAt: o.ReadyAt}
		if p, ok := s.st.Products[o.GTIN]; ok {
			v.Product = p.Name
		}
		snap.Orders = append(snap.Orders, v)
	}
	sort.Slice(snap.Orders, func(i, j int) bool { return snap.Orders[i].CreatedAt.After(snap.Orders[j].CreatedAt) })
	for _, d := range s.documents("", 50) {
		snap.Documents = append(snap.Documents, DocumentView{
			ID: d.ID, Number: d.Number, Type: d.Type, TypeTitle: DocumentTypes[d.Type].Title, Status: d.Status,
			SenderBIN: d.SenderBIN, Codes: len(d.Codes), ReceivedAt: d.ReceivedAt, ProcessedAt: d.ProcessedAt,
			ErrorMessage: d.ErrorMessage, CisErrors: d.CisErrors,
		})
	}
	return snap
}
