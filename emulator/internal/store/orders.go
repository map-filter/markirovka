package store

import (
	"net/http"
	"sort"
	"strings"

	"markirovka/emulator/internal/gs1"
)

type OrderRequest struct {
	GTIN        string
	PackageType string
	Quantity    *int
}

func (s *Store) CreateOrder(bin string, req OrderRequest) (string, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	gtin := strings.TrimSpace(req.GTIN)
	if gtin == "" {
		return "", newError(http.StatusBadRequest, "GTIN должен быть указан в параметрах запроса")
	}
	if err := gs1.ValidateGTIN(gtin); err != nil {
		return "", newError(http.StatusBadRequest, "Неверный формат GTIN: "+err.Error())
	}
	pkg := req.PackageType
	if pkg == "" {
		pkg = PackageUnit
	}
	if pkg != PackageUnit {
		return "", newError(http.StatusBadRequest, "Поддерживается только packageType UNIT")
	}
	p, ok := s.st.Products[gtin]
	if !ok || p.OwnerBIN != bin {
		return "", newError(http.StatusNotFound, "Товар с GTIN "+gtin+" не найден в НКТ участника")
	}
	now := s.realNow()
	o := &Order{
		UUID:        NewUUID(),
		OwnerBIN:    bin,
		Kind:        OrderKindList,
		GTIN:        gtin,
		PackageType: pkg,
		CreatedAt:   now,
		ReadyAt:     now.Add(s.cfg.OrderDelay),
	}
	if req.Quantity != nil {
		q := *req.Quantity
		if q < 1 || q > MaxOrderQuantity {
			return "", newError(http.StatusBadRequest, "Параметр quantity требует значение от 1 до 10000")
		}
		o.Kind = OrderKindEmission
		o.Quantity = q
		o.Codes = s.emit(bin, gtin, o.UUID, q)
	}
	s.st.Orders[o.UUID] = o
	return o.UUID, s.save()
}

func (s *Store) emit(bin, gtin, orderUUID string, n int) []string {
	emu := s.emuNow()
	out := make([]string, 0, n)
	for len(out) < n {
		serial := gs1.RandomSerial()
		ki := "01" + gtin + "21" + serial
		if _, dup := s.st.Codes[ki]; dup {
			continue
		}
		key, crypto := s.cryptoTail(ki)
		s.st.Codes[ki] = &Code{
			KI:        ki,
			GTIN:      gtin,
			Serial:    serial,
			Key:       key,
			Crypto:    crypto,
			OwnerBIN:  bin,
			Status:    StatusEmitted,
			OrderUUID: orderUUID,
			EmittedAt: emu,
		}
		out = append(out, ki)
	}
	return out
}

func (s *Store) order(bin, uuid string) (*Order, error) {
	o, ok := s.st.Orders[uuid]
	if !ok || o.OwnerBIN != bin {
		return nil, newError(http.StatusNotFound, "Заказ с идентификатором "+uuid+" не найден")
	}
	return o, nil
}

func (s *Store) orderStatus(o *Order) string {
	if s.realNow().Before(o.ReadyAt) {
		return OrderInProgress
	}
	return OrderSuccess
}

func (s *Store) OrderStatus(bin, uuid string) (string, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	o, err := s.order(bin, uuid)
	if err != nil {
		return "", err
	}
	return s.orderStatus(o), nil
}

func (s *Store) OrderResult(bin, uuid string) ([]CisRecord, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	o, err := s.order(bin, uuid)
	if err != nil {
		return nil, err
	}
	if s.orderStatus(o) != OrderSuccess {
		return nil, newError(http.StatusConflict, "Заказ с идентификатором "+uuid+" еще не выполнен")
	}
	if s.advance() {
		_ = s.save()
	}
	keys := o.Codes
	if o.Kind == OrderKindList {
		keys = keys[:0:0]
		for ki, c := range s.st.Codes {
			if c.OwnerBIN == bin && c.GTIN == o.GTIN {
				keys = append(keys, ki)
			}
		}
		sort.Strings(keys)
		if len(keys) > MaxOrderQuantity {
			keys = keys[:MaxOrderQuantity]
		}
	}
	out := make([]CisRecord, 0, len(keys))
	for _, ki := range keys {
		if c, ok := s.st.Codes[ki]; ok {
			out = append(out, s.record(c))
		}
	}
	return out, nil
}
