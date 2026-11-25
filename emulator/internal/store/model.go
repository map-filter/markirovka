package store

import (
	"encoding/json"
	"time"
)

const (
	StatusEmitted    = "EMITTED"
	StatusApplied    = "APPLIED"
	StatusIntroduced = "INTRODUCED"
	StatusWithdrawn  = "WITHDRAWN"
	StatusWrittenOff = "WRITTEN_OFF"

	ExtAnnulled = "ANNULLED"

	OrderInProgress = "IN_PROGRESS"
	OrderSuccess    = "SUCCESS"

	OrderKindEmission = "EMISSION"
	OrderKindList     = "LIST"

	DocInProgress   = "IN_PROGRESS"
	DocCheckedOK    = "CHECKED_OK"
	DocCheckedNotOK = "CHECKED_NOT_OK"

	DocApplicationReport = "KZ_APPLICATION_REPORT"
	DocIntroduceLocal    = "KZ_LP_INTRODUCE_GOODS"
	DocWithdraw          = "MTTPS_90"

	PackageUnit      = "UNIT"
	ProductGroup     = "lp"
	ProductGroupID   = 1
	MaxOrderQuantity = 10000
)

var DocumentTypes = map[string]struct {
	From, To, Title string
}{
	DocApplicationReport: {StatusEmitted, StatusApplied, "Отчёт о нанесении"},
	DocIntroduceLocal:    {StatusApplied, StatusIntroduced, "Ввод в оборот (производство РК)"},
	DocWithdraw:          {StatusIntroduced, StatusWithdrawn, "Вывод из оборота"},
}

var StatusTitles = map[string]string{
	StatusEmitted:    "Эмитирован",
	StatusApplied:    "Нанесён",
	StatusIntroduced: "Введен в оборот",
	StatusWithdrawn:  "Выведен из оборота",
	StatusWrittenOff: "Утилизирован (аннулирован)",
}

type Participant struct {
	BIN     string `json:"bin"`
	Name    string `json:"name"`
	Address string `json:"address"`
}

type Product struct {
	ID          int64     `json:"id"`
	GTIN        string    `json:"gtin"`
	Name        string    `json:"name"`
	Model       string    `json:"model"`
	Size        string    `json:"size"`
	Color       string    `json:"color"`
	Composition string    `json:"composition"`
	TNVED       string    `json:"tnved"`
	Category    string    `json:"category"`
	Brand       string    `json:"brand"`
	Country     string    `json:"country"`
	OwnerBIN    string    `json:"ownerBin"`
	CreatedAt   time.Time `json:"createdAt"`
}

type Seed struct {
	Participant Participant `json:"participant"`
	Products    []Product   `json:"products"`
}

type Code struct {
	KI           string     `json:"ki"`
	GTIN         string     `json:"gtin"`
	Serial       string     `json:"serial"`
	Key          string     `json:"key"`
	Crypto       string     `json:"crypto"`
	OwnerBIN     string     `json:"ownerBin"`
	Status       string     `json:"status"`
	Annulled     bool       `json:"annulled,omitempty"`
	OrderUUID    string     `json:"orderUuid"`
	EmittedAt    time.Time  `json:"emittedAt"`
	AppliedAt    *time.Time `json:"appliedAt,omitempty"`
	IntroducedAt *time.Time `json:"introducedAt,omitempty"`
	WithdrawnAt  *time.Time `json:"withdrawnAt,omitempty"`
	AnnulledAt   *time.Time `json:"annulledAt,omitempty"`
}

type Order struct {
	UUID        string    `json:"uuid"`
	OwnerBIN    string    `json:"ownerBin"`
	Kind        string    `json:"kind"`
	GTIN        string    `json:"gtin"`
	PackageType string    `json:"packageType"`
	Quantity    int       `json:"quantity"`
	CreatedAt   time.Time `json:"createdAt"`
	ReadyAt     time.Time `json:"readyAt"`
	Codes       []string  `json:"codes,omitempty"`
}

type CisError struct {
	Cis          string `json:"cis"`
	ErrorCode    string `json:"errorCode"`
	ErrorMessage string `json:"errorMessage"`
}

type Document struct {
	ID           string          `json:"id"`
	Number       string          `json:"number"`
	Type         string          `json:"type"`
	Status       string          `json:"status"`
	SenderBIN    string          `json:"senderBin"`
	SenderName   string          `json:"senderName"`
	DocDate      time.Time       `json:"docDate"`
	ReceivedAt   time.Time       `json:"receivedAt"`
	ProcessAt    time.Time       `json:"processAt"`
	ProcessedAt  *time.Time      `json:"processedAt,omitempty"`
	Content      string          `json:"content"`
	Body         json.RawMessage `json:"body"`
	Codes        []string        `json:"codes"`
	CisErrors    []CisError      `json:"cisErrors,omitempty"`
	ErrorMessage string          `json:"errorMessage,omitempty"`
}

type State struct {
	Secret        string                  `json:"secret"`
	SeedBIN       string                  `json:"seedBin"`
	OffsetSec     int64                   `json:"offsetSec"`
	Participants  map[string]*Participant `json:"participants"`
	Products      map[string]*Product     `json:"products"`
	Codes         map[string]*Code        `json:"codes"`
	Orders        map[string]*Order       `json:"orders"`
	Documents     map[string]*Document    `json:"documents"`
	DocSeq        int                     `json:"docSeq"`
	NextProductID int64                   `json:"nextProductId"`
}

type Error struct {
	Status  int
	Message string
}

func (e *Error) Error() string { return e.Message }

func newError(status int, msg string) *Error {
	return &Error{Status: status, Message: msg}
}
