package store

import (
	"net/http"
	"sort"
	"strings"
	"time"

	"markirovka/emulator/internal/gs1"
)

type CisRecord struct {
	Cis          string     `json:"cis"`
	Code         string     `json:"code"`
	PrintCode    string     `json:"printCode,omitempty"`
	HumanCode    string     `json:"humanCode,omitempty"`
	GTIN         string     `json:"gtin"`
	ProducedDate *time.Time `json:"producedDate,omitempty"`
	PackageType  string     `json:"packageType"`
	Status       string     `json:"status"`
	EmissionDate time.Time  `json:"emissionDate"`
}

type CisInfo struct {
	RequestedCis   string     `json:"requestedCis"`
	Cis            string     `json:"cis,omitempty"`
	GTIN           string     `json:"gtin,omitempty"`
	ProductGroupID int        `json:"productGroupId,omitempty"`
	ProductGroup   string     `json:"productGroup,omitempty"`
	ProductName    string     `json:"productName,omitempty"`
	Brand          string     `json:"brand,omitempty"`
	ProducedDate   *time.Time `json:"producedDate,omitempty"`
	PackageType    string     `json:"packageType,omitempty"`
	OwnerBIN       string     `json:"ownerBin,omitempty"`
	OwnerName      string     `json:"ownerName,omitempty"`
	Status         string     `json:"status,omitempty"`
	StatusExt      string     `json:"statusExt,omitempty"`
	ProducerBIN    string     `json:"producerBin,omitempty"`
	ProducerName   string     `json:"producerName,omitempty"`
	EmissionDate   *time.Time `json:"emissionDate,omitempty"`
	ApplicationAt  *time.Time `json:"applicationDate,omitempty"`
	WithdrawnAt    *time.Time `json:"withdrawalDate,omitempty"`
}

type CisInfoItem struct {
	CisInfo      CisInfo `json:"cisInfo"`
	ErrorCode    string  `json:"errorCode,omitempty"`
	ErrorMessage string  `json:"errorMessage,omitempty"`
}

func (s *Store) Product(gtin string) (Product, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	p, ok := s.st.Products[gtin]
	if !ok {
		return Product{}, false
	}
	return *p, true
}

func (s *Store) ProductsOf(bin string) []Product {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.productsOf(bin)
}

func (s *Store) productsOf(bin string) []Product {
	out := make([]Product, 0)
	for _, p := range s.st.Products {
		if bin == "" || p.OwnerBIN == bin {
			out = append(out, *p)
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].GTIN < out[j].GTIN })
	return out
}

func (s *Store) AddProduct(p Product) (Product, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	p.GTIN = strings.TrimSpace(p.GTIN)
	if err := gs1.ValidateGTIN(p.GTIN); err != nil {
		return Product{}, newError(http.StatusBadRequest, err.Error())
	}
	if strings.TrimSpace(p.Name) == "" {
		return Product{}, newError(http.StatusBadRequest, "Отсутствует обязательный параметр: name")
	}
	if p.OwnerBIN == "" {
		p.OwnerBIN = s.seedBIN()
	}
	if _, ok := s.st.Participants[p.OwnerBIN]; !ok {
		return Product{}, newError(http.StatusBadRequest, "Участник с ИИН/БИН "+p.OwnerBIN+" не найден")
	}
	if _, ok := s.st.Products[p.GTIN]; ok {
		return Product{}, newError(http.StatusConflict, "Карточка с GTIN "+p.GTIN+" уже зарегистрирована")
	}
	s.st.NextProductID++
	p.ID = s.st.NextProductID
	p.CreatedAt = s.realNow()
	s.st.Products[p.GTIN] = &p
	return p, s.save()
}

func (s *Store) record(c *Code) CisRecord {
	code := gs1.Code{GTIN: c.GTIN, Serial: c.Serial, Key: c.Key, Crypto: c.Crypto}
	return CisRecord{
		Cis:          code.CIS(),
		Code:         code.KI(),
		PrintCode:    code.Raw(),
		HumanCode:    code.HumanReadable(),
		GTIN:         c.GTIN,
		ProducedDate: c.IntroducedAt,
		PackageType:  PackageUnit,
		Status:       c.Status,
		EmissionDate: c.EmittedAt,
	}
}

func (s *Store) CisInfo(bin string, requested []string) ([]CisInfoItem, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if len(requested) == 0 {
		return nil, newError(http.StatusBadRequest, "В запросе не указан ни один КМ")
	}
	if len(requested) > 1000 {
		return nil, newError(http.StatusBadRequest, "Не более 1000 КИ в массиве")
	}
	if s.advance() {
		_ = s.save()
	}
	out := make([]CisInfoItem, 0, len(requested))
	found := false
	for _, req := range requested {
		parsed, err := gs1.Parse(req)
		if err != nil {
			found = true
			out = append(out, CisInfoItem{CisInfo: CisInfo{RequestedCis: req}, ErrorCode: "400", ErrorMessage: err.Error()})
			continue
		}
		c, ok := s.st.Codes[parsed.KI()]
		if !ok {
			out = append(out, CisInfoItem{CisInfo: CisInfo{RequestedCis: req}, ErrorCode: "404", ErrorMessage: "КМ не найден"})
			continue
		}
		if c.OwnerBIN != bin {
			continue
		}
		found = true
		out = append(out, CisInfoItem{CisInfo: s.info(req, c)})
	}
	if !found {
		return nil, newError(http.StatusNotFound, "КМ не найдены")
	}
	return out, nil
}

func (s *Store) info(req string, c *Code) CisInfo {
	code := gs1.Code{GTIN: c.GTIN, Serial: c.Serial}
	info := CisInfo{
		RequestedCis:   req,
		Cis:            code.CIS(),
		GTIN:           c.GTIN,
		ProductGroupID: ProductGroupID,
		ProductGroup:   ProductGroup,
		ProducedDate:   c.IntroducedAt,
		PackageType:    PackageUnit,
		OwnerBIN:       c.OwnerBIN,
		Status:         c.Status,
		ProducerBIN:    c.OwnerBIN,
		ApplicationAt:  c.AppliedAt,
		WithdrawnAt:    c.WithdrawnAt,
	}
	emitted := c.EmittedAt
	info.EmissionDate = &emitted
	if c.Annulled {
		info.StatusExt = ExtAnnulled
	}
	if p, ok := s.st.Products[c.GTIN]; ok {
		info.ProductName = p.Name
		info.Brand = p.Brand
	}
	if p, ok := s.st.Participants[c.OwnerBIN]; ok {
		info.OwnerName = p.Name
		info.ProducerName = p.Name
	}
	return info
}
