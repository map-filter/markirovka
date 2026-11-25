package server

import (
	"archive/zip"
	"bytes"
	"encoding/json"
	"net/http"
	"regexp"
	"strconv"
	"strings"

	"markirovka/emulator/internal/gs1"
	"markirovka/emulator/internal/sign"
	"markirovka/emulator/internal/store"
)

var gtinPattern = regexp.MustCompile(`^\d{14}$`)

func (s *Server) handleProductGTIN(w http.ResponseWriter, r *http.Request, bin string) {
	q := r.URL.Query()
	pg := q.Get("pg")
	if pg == "" {
		writeError(w, http.StatusBadRequest, "Отсутствует обязательный параметр: pg")
		return
	}
	if pg != store.ProductGroup {
		writeError(w, http.StatusBadRequest, "Could not determine product group by name '"+pg+"'")
		return
	}
	limit, ok := intParam(w, q.Get("limit"), "limit", 10)
	if !ok {
		return
	}
	page, ok := intParam(w, q.Get("page"), "page", 0)
	if !ok {
		return
	}
	limit = min(max(limit, 1), 10000)
	page = max(page, 0)
	products := s.store.ProductsOf(bin)
	gtins := make([]string, 0, limit)
	for i := page * limit; i < len(products) && len(gtins) < limit; i++ {
		gtins = append(gtins, products[i].GTIN)
	}
	writeJSON(w, http.StatusOK, map[string]any{"gtins": gtins, "total": len(products)})
}

func intParam(w http.ResponseWriter, v, name string, def int) (int, bool) {
	if v == "" {
		return def, true
	}
	n, err := strconv.Atoi(v)
	if err != nil {
		writeError(w, http.StatusBadRequest, "Параметр "+name+" требует значение типа Number")
		return 0, false
	}
	return n, true
}

func (s *Server) handleProductsInfo(w http.ResponseWriter, r *http.Request, bin string) {
	q := r.URL.Query()
	in := strings.TrimSpace(q.Get("cis"))
	if in == "" {
		in = strings.TrimSpace(q.Get("gtin"))
	}
	if in == "" {
		writeError(w, http.StatusBadRequest, "Отсутствует обязательный параметр: cis")
		return
	}
	gtin := in
	if !gtinPattern.MatchString(in) {
		c, err := gs1.Parse(in)
		if err != nil {
			writeError(w, http.StatusBadRequest, "Неверный формат КМ")
			return
		}
		gtin = c.GTIN
	}
	p, ok := s.store.Product(gtin)
	if !ok {
		writeError(w, http.StatusNotFound, "Продукт не найден")
		return
	}
	writeJSON(w, http.StatusOK, productInfo(p))
}

func productInfo(p store.Product) map[string]any {
	return map[string]any{
		"id":             p.ID,
		"name":           p.Name,
		"gtin":           p.GTIN,
		"packageType":    "trade-unit",
		"innerUnitCount": 1,
		"productGroupId": store.ProductGroupID,
		"productGroup":   store.ProductGroup,
		"ownerBin":       p.OwnerBIN,
		"specificAttributes": map[string]string{
			"model":       p.Model,
			"size":        p.Size,
			"color":       p.Color,
			"composition": p.Composition,
			"tnved":       p.TNVED,
			"category":    p.Category,
			"brand":       p.Brand,
			"country":     p.Country,
		},
	}
}

func (s *Server) handleCisesMy(w http.ResponseWriter, r *http.Request, bin string) {
	var req struct {
		GTIN         string          `json:"gtin"`
		PackageType  string          `json:"packageType"`
		ProducedDate string          `json:"producedDate"`
		Quantity     json.RawMessage `json:"quantity"`
	}
	if !decodeJSON(w, r, &req) {
		return
	}
	or := store.OrderRequest{GTIN: req.GTIN, PackageType: req.PackageType}
	if len(req.Quantity) > 0 && string(req.Quantity) != "null" {
		var q int
		if err := json.Unmarshal(req.Quantity, &q); err != nil {
			writeError(w, http.StatusBadRequest, "Параметр quantity требует значение типа Number")
			return
		}
		or.Quantity = &q
	}
	id, err := s.store.CreateOrder(bin, or)
	if err != nil {
		writeStoreError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"uuid": id})
}

func (s *Server) handleOrderStatus(w http.ResponseWriter, r *http.Request, bin string) {
	status, err := s.store.OrderStatus(bin, r.PathValue("uuid"))
	if err != nil {
		writeStoreError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": status})
}

func (s *Server) handleOrderResult(w http.ResponseWriter, r *http.Request, bin string) {
	id := r.PathValue("uuid")
	records, err := s.store.OrderResult(bin, id)
	if err != nil {
		writeStoreError(w, err)
		return
	}
	if wantsJSONOnly(r.Header.Values("Accept")) {
		writeJSON(w, http.StatusOK, records)
		return
	}
	var payload bytes.Buffer
	enc := json.NewEncoder(&payload)
	enc.SetEscapeHTML(false)
	if err := enc.Encode(records); err != nil {
		writeStoreError(w, err)
		return
	}
	var buf bytes.Buffer
	zw := zip.NewWriter(&buf)
	f, err := zw.Create(id + ".json")
	if err == nil {
		_, err = f.Write(payload.Bytes())
	}
	if err == nil {
		err = zw.Close()
	}
	if err != nil {
		writeStoreError(w, err)
		return
	}
	w.Header().Set("Content-Type", "application/zip")
	w.Header().Set("Content-Disposition", `attachment; filename="`+id+`.zip"`)
	w.WriteHeader(http.StatusOK)
	w.Write(buf.Bytes())
}

func wantsJSONOnly(accept []string) bool {
	joined := strings.ToLower(strings.Join(accept, ","))
	return strings.Contains(joined, "application/json") && !strings.Contains(joined, "zip") &&
		!strings.Contains(joined, "*/*") && !strings.Contains(joined, "octet-stream")
}

func (s *Server) handleCisesInfo(w http.ResponseWriter, r *http.Request, bin string) {
	var codes []string
	if !decodeJSON(w, r, &codes) {
		return
	}
	items, err := s.store.CisInfo(bin, codes)
	if err != nil {
		writeStoreError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, items)
}

func (s *Server) handleDocumentCreate(w http.ResponseWriter, r *http.Request, bin string) {
	var req struct {
		Type      string `json:"type"`
		Document  string `json:"document"`
		Signature string `json:"signature"`
	}
	if !decodeJSON(w, r, &req) {
		return
	}
	if req.Type == "" {
		req.Type = r.URL.Query().Get("type")
	}
	if req.Type == "" {
		writeError(w, http.StatusBadRequest, "Отсутствует обязательный параметр: type")
		return
	}
	s.createDocument(w, bin, req.Type, req.Document, req.Signature, "document")
}

func (s *Server) handleWriteOff(w http.ResponseWriter, r *http.Request, bin string) {
	var req struct {
		Document  string `json:"document"`
		Signature string `json:"signature"`
	}
	if !decodeJSON(w, r, &req) {
		return
	}
	s.createDocument(w, bin, store.DocWithdraw, req.Document, req.Signature, "document")
}

func (s *Server) handleFormal(w http.ResponseWriter, r *http.Request, bin string) {
	if err := r.ParseMultipartForm(maxBodyLen); err != nil {
		if err := r.ParseForm(); err != nil {
			writeError(w, http.StatusUnsupportedMediaType, "Формат содержимого не поддерживается сервером")
			return
		}
	}
	docType := r.FormValue("type")
	if docType == "" {
		writeError(w, http.StatusBadRequest, "Отсутствует обязательный параметр: type")
		return
	}
	s.createDocument(w, bin, docType, r.FormValue("content"), r.FormValue("signature"), "content")
}

func (s *Server) createDocument(w http.ResponseWriter, bin, docType, content, signature, contentField string) {
	if _, ok := store.DocumentTypes[docType]; !ok {
		writeError(w, http.StatusBadRequest, "Неизвестный тип документа: "+docType)
		return
	}
	if content == "" {
		writeError(w, http.StatusBadRequest, "Отсутствует обязательный параметр: "+contentField)
		return
	}
	if signature == "" {
		writeError(w, http.StatusBadRequest, "Отсутствует обязательный параметр: signature")
		return
	}
	body, err := sign.DecodeBase64(content)
	if err != nil || len(body) == 0 {
		writeError(w, http.StatusBadRequest, "Параметр "+contentField+" требует значение типа String, Base64")
		return
	}
	res, err := sign.Verify(signature, s.signOpts())
	if err != nil {
		writeError(w, http.StatusUnprocessableEntity, "Ошибка проверки подписи: "+err.Error())
		return
	}
	if res.Emulated {
		if !bytes.Equal(res.Data, body) {
			writeError(w, http.StatusUnprocessableEntity, "Ошибка проверки подписи: подписанные данные не совпадают с документом")
			return
		}
		if res.BIN != bin {
			writeError(w, http.StatusUnprocessableEntity, "Ошибка проверки подписи: ИИН/БИН подписанта "+res.BIN+" не совпадает с токеном")
			return
		}
	}
	id, err := s.store.CreateDocument(bin, store.NewDocument{Type: docType, Content: content, Body: body})
	if err != nil {
		writeStoreError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"documentId": id})
}

func (s *Server) handleDocumentInfo(w http.ResponseWriter, r *http.Request, bin string) {
	a, b := r.PathValue("a"), r.PathValue("b")
	var id string
	switch {
	case a == "info":
		id = b
	case b == "info":
		id = a
	default:
		notFound(w, r)
		return
	}
	d, err := s.store.Document(bin, id)
	if err != nil {
		writeStoreError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, documentInfo(d))
}

func documentInfo(d store.Document) map[string]any {
	var body any
	_ = json.Unmarshal(d.Body, &body)
	out := map[string]any{
		"documentId":     d.ID,
		"number":         d.Number,
		"docDate":        d.DocDate,
		"receivedAt":     d.ReceivedAt,
		"type":           d.Type,
		"status":         d.Status,
		"externalId":     d.ID,
		"senderBin":      d.SenderBIN,
		"senderName":     d.SenderName,
		"receiverBin":    "",
		"receiverName":   "",
		"downloadStatus": d.Status,
		"body":           body,
		"content":        d.Content,
		"input":          false,
		"errors":         d.Errors(),
		"cisErrors":      cisErrors(d.CisErrors),
	}
	if d.ProcessedAt != nil {
		out["processedAt"] = d.ProcessedAt
	}
	if d.ErrorMessage != "" {
		out["errorMessage"] = d.ErrorMessage
	}
	return out
}

func cisErrors(e []store.CisError) []store.CisError {
	if e == nil {
		return []store.CisError{}
	}
	return e
}

func (s *Server) handleDocList(w http.ResponseWriter, r *http.Request, bin string) {
	q := r.URL.Query()
	limit, ok := intParam(w, q.Get("limit"), "limit", 10)
	if !ok {
		return
	}
	limit = min(max(limit, 1), 10000)
	status := q.Get("documentStatus")
	types := q["documentType"]
	results := make([]map[string]any, 0)
	for _, d := range s.store.Documents(bin, 0) {
		if status != "" && d.Status != status {
			continue
		}
		if len(types) > 0 && !contains(types, d.Type) {
			continue
		}
		info := documentInfo(d)
		delete(info, "content")
		delete(info, "body")
		results = append(results, info)
		if len(results) >= limit {
			break
		}
	}
	writeJSON(w, http.StatusOK, map[string]any{"results": results, "total": len(results)})
}

func contains(list []string, v string) bool {
	for _, x := range list {
		if x == v {
			return true
		}
	}
	return false
}
