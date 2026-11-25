package server

import (
	"archive/zip"
	"bytes"
	"encoding/base64"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"markirovka/emulator/internal/sign"
	"markirovka/emulator/internal/store"
)

const (
	testBIN    = "190440015237"
	gtinShirt  = "04870712001017"
	gtinHoodie = "04870712001079"
)

const seedJSON = `{
 "participant": {"bin": "190440015237", "name": "ТОО «Кендір Тігін»", "address": "г. Павлодар"},
 "products": [
  {"gtin": "04870712001017", "name": "Футболка мужская базовая, белый, р. M", "model": "FT-M-01", "size": "M", "color": "белый", "composition": "хлопок 100%", "tnved": "6109100000", "category": "футболки", "brand": "Кендір", "country": "Казахстан"},
  {"gtin": "04870712001024", "name": "Футболка мужская базовая, белый, р. L", "model": "FT-M-01", "size": "L", "color": "белый", "composition": "хлопок 100%", "tnved": "6109100000", "category": "футболки", "brand": "Кендір", "country": "Казахстан"},
  {"gtin": "04870712001079", "name": "Худи унисекс", "model": "HD-U-10", "size": "M", "color": "серый", "composition": "хлопок 80%", "tnved": "6110209900", "category": "худи", "brand": "Кендір", "country": "Казахстан"}
 ]
}`

type fakeClock struct {
	mu sync.Mutex
	t  time.Time
}

func (c *fakeClock) Now() time.Time {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.t
}

func (c *fakeClock) Add(d time.Duration) {
	c.mu.Lock()
	c.t = c.t.Add(d)
	c.mu.Unlock()
}

type env struct {
	t     *testing.T
	dir   string
	clock *fakeClock
	cfg   Config
	store *store.Store
	srv   *Server
}

func newEnv(t *testing.T) *env {
	t.Helper()
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "seed.json"), []byte(seedJSON), 0o644); err != nil {
		t.Fatal(err)
	}
	e := &env{t: t, dir: dir, clock: &fakeClock{t: time.Date(2026, 11, 25, 9, 0, 0, 0, time.UTC)},
		cfg: Config{AcceptEmulatedSignature: true}}
	e.open()
	return e
}

func (e *env) open() {
	e.t.Helper()
	st, err := store.New(store.Config{
		SeedFile:   filepath.Join(e.dir, "seed.json"),
		StateFile:  filepath.Join(e.dir, "data", "state.json"),
		OrderDelay: 5 * time.Second,
		DocDelay:   2 * time.Second,
		AnnulDays:  30,
	}, e.clock.Now)
	if err != nil {
		e.t.Fatal(err)
	}
	e.store = st
	e.srv = New(e.cfg, st, e.clock.Now)
}

func (e *env) do(method, path, token string, body any, headers ...string) *httptest.ResponseRecorder {
	e.t.Helper()
	var rd io.Reader
	switch b := body.(type) {
	case nil:
	case string:
		rd = strings.NewReader(b)
	case []byte:
		rd = bytes.NewReader(b)
	default:
		data, err := json.Marshal(b)
		if err != nil {
			e.t.Fatal(err)
		}
		rd = bytes.NewReader(data)
	}
	req := httptest.NewRequest(method, path, rd)
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	for i := 0; i+1 < len(headers); i += 2 {
		req.Header.Set(headers[i], headers[i+1])
	}
	rec := httptest.NewRecorder()
	e.srv.ServeHTTP(rec, req)
	return rec
}

func decode[T any](t *testing.T, rec *httptest.ResponseRecorder) T {
	t.Helper()
	var v T
	if err := json.Unmarshal(rec.Body.Bytes(), &v); err != nil {
		t.Fatalf("decode %q: %v", rec.Body.String(), err)
	}
	return v
}

func expect(t *testing.T, rec *httptest.ResponseRecorder, status int) {
	t.Helper()
	if rec.Code != status {
		t.Fatalf("status %d, want %d, body %s", rec.Code, status, rec.Body.String())
	}
}

func (e *env) authKey() (string, string) {
	rec := e.do("GET", BasePath+"/auth/key", "", nil)
	expect(e.t, rec, 200)
	k := decode[map[string]string](e.t, rec)
	if k["uuid"] == "" || len(k["data"]) != 30 {
		e.t.Fatalf("bad key %v", k)
	}
	return k["uuid"], k["data"]
}

func (e *env) login() string {
	e.t.Helper()
	id, data := e.authKey()
	rec := e.do("POST", BasePath+"/auth/simpleSignIn", "", map[string]string{"uuid": id, "data": sign.Emulate([]byte(data), testBIN)})
	expect(e.t, rec, 200)
	return decode[map[string]string](e.t, rec)["token"]
}

func (e *env) order(token, gtin string, qty int) []store.CisRecord {
	e.t.Helper()
	rec := e.do("POST", BasePath+"/cises/my", token, map[string]any{"gtin": gtin, "quantity": qty})
	expect(e.t, rec, 200)
	id := decode[map[string]string](e.t, rec)["uuid"]
	e.clock.Add(6 * time.Second)
	rec = e.do("GET", BasePath+"/cises/orders/"+id+"/result", token, nil, "Accept", "application/json")
	expect(e.t, rec, 200)
	return decode[[]store.CisRecord](e.t, rec)
}

func docPayload(codes ...string) []byte {
	list := make([]map[string]any, 0, len(codes))
	for _, c := range codes {
		list = append(list, map[string]any{"iCode": c})
	}
	data, _ := json.Marshal(map[string]any{
		"version":         "1.0",
		"sendingDateTime": "2026-11-25T09:00:00Z",
		"document": map[string]any{
			"mainInfo": map[string]any{
				"reason":          "1",
				"participantInfo": map[string]string{"participantIdentificationCode": testBIN},
			},
			"productList": list,
		},
	})
	return data
}

func (e *env) sendDoc(token, docType string, codes ...string) string {
	e.t.Helper()
	body := docPayload(codes...)
	rec := e.do("POST", BasePath+"/documents/create", token, map[string]string{
		"type":      docType,
		"document":  base64.StdEncoding.EncodeToString(body),
		"signature": sign.Emulate(body, testBIN),
	})
	expect(e.t, rec, 200)
	return decode[map[string]string](e.t, rec)["documentId"]
}

func (e *env) docInfo(token, id string) map[string]any {
	e.t.Helper()
	rec := e.do("GET", BasePath+"/documents/"+id+"/info", token, nil)
	expect(e.t, rec, 200)
	return decode[map[string]any](e.t, rec)
}

func (e *env) processedDoc(token, id string) map[string]any {
	e.t.Helper()
	if s := e.docInfo(token, id)["status"]; s != "IN_PROGRESS" {
		e.t.Fatalf("expected IN_PROGRESS right after creation, got %v", s)
	}
	e.clock.Add(3 * time.Second)
	return e.docInfo(token, id)
}

func (e *env) statuses(token string, codes ...string) map[string]store.CisInfoItem {
	e.t.Helper()
	rec := e.do("POST", BasePath+"/cises/info", token, codes)
	expect(e.t, rec, 200)
	out := map[string]store.CisInfoItem{}
	for _, it := range decode[[]store.CisInfoItem](e.t, rec) {
		out[it.CisInfo.RequestedCis] = it
	}
	return out
}

func TestAuthFlow(t *testing.T) {
	e := newEnv(t)
	token := e.login()
	if strings.Count(token, ".") != 2 {
		t.Fatalf("token is not JWT-shaped: %s", token)
	}
	expect(t, e.do("GET", BasePath+"/product/gtin?pg=lp", token, nil), 200)

	rec := e.do("GET", BasePath+"/product/gtin?pg=lp", "", nil)
	expect(t, rec, 401)
	if !strings.Contains(rec.Body.String(), "<UnauthorizedException>") || !strings.Contains(rec.Header().Get("Content-Type"), "xml") {
		t.Fatalf("unexpected 401 body %s", rec.Body.String())
	}
	rec = e.do("GET", BasePath+"/product/gtin?pg=lp", "garbage.token.x", nil)
	expect(t, rec, 401)
	if !strings.Contains(rec.Body.String(), "<InvalidTokenException>") {
		t.Fatalf("unexpected body %s", rec.Body.String())
	}

	id, data := e.authKey()
	rec = e.do("POST", BasePath+"/auth/simpleSignIn", "", map[string]string{"uuid": id, "data": sign.Emulate([]byte(data+"X"), testBIN)})
	expect(t, rec, 401)
	if decode[map[string]string](t, rec)["error_message"] != authFailMsg {
		t.Fatalf("body %s", rec.Body.String())
	}
	rec = e.do("POST", BasePath+"/auth/simpleSignIn", "", map[string]string{"uuid": id, "data": sign.Emulate([]byte(data), testBIN)})
	expect(t, rec, 401)

	id, data = e.authKey()
	expect(t, e.do("POST", BasePath+"/auth/simpleSignIn", "", map[string]string{"uuid": id, "data": sign.Emulate([]byte(data), "000000000000")}), 401)
	expect(t, e.do("POST", BasePath+"/auth/simpleSignIn", "", map[string]string{"uuid": id}), 400)

	id, _ = e.authKey()
	realCMS := base64.StdEncoding.EncodeToString([]byte{0x30, 0x82, 0x01, 0x02, 0x03})
	expect(t, e.do("POST", BasePath+"/auth/simpleSignIn", "", map[string]string{"uuid": id, "data": realCMS}), 401)

	e.clock.Add(10*time.Hour + time.Second)
	rec = e.do("GET", BasePath+"/product/gtin?pg=lp", token, nil)
	expect(t, rec, 401)
	if !strings.Contains(rec.Body.String(), "Access token expired: "+token) {
		t.Fatalf("unexpected body %s", rec.Body.String())
	}
}

func TestAnyCMSMode(t *testing.T) {
	e := newEnv(t)
	e.cfg.AcceptAnyCMS = true
	e.srv = New(e.cfg, e.store, e.clock.Now)
	id, _ := e.authKey()
	realCMS := base64.StdEncoding.EncodeToString([]byte{0x30, 0x82, 0x01, 0x02, 0x03})
	rec := e.do("POST", BasePath+"/auth/simpleSignIn", "", map[string]string{"uuid": id, "data": realCMS})
	expect(t, rec, 200)
}

func TestProducts(t *testing.T) {
	e := newEnv(t)
	token := e.login()

	rec := e.do("GET", BasePath+"/product/gtin?pg=lp&limit=2&page=0", token, nil)
	expect(t, rec, 200)
	list := decode[struct {
		GTINs []string `json:"gtins"`
		Total int      `json:"total"`
	}](t, rec)
	if list.Total != 3 || len(list.GTINs) != 2 || list.GTINs[0] != gtinShirt {
		t.Fatalf("got %+v", list)
	}
	expect(t, e.do("GET", BasePath+"/product/gtin", token, nil), 400)
	expect(t, e.do("GET", BasePath+"/product/gtin?pg=shoe", token, nil), 400)

	rec = e.do("GET", BasePath+"/products/info?cis="+gtinShirt, token, nil)
	expect(t, rec, 200)
	info := decode[map[string]any](t, rec)
	if info["gtin"] != gtinShirt || info["packageType"] != "trade-unit" || info["specificAttributes"].(map[string]any)["size"] != "M" {
		t.Fatalf("got %v", info)
	}
	expect(t, e.do("GET", BasePath+"/products/info?cis=%2801%29"+gtinShirt+"%2821%29abcdefghijklm", token, nil), 200)
	expect(t, e.do("GET", BasePath+"/products/info?cis=04870712009990", token, nil), 404)
	expect(t, e.do("GET", BasePath+"/products/info?cis=nonsense", token, nil), 400)

	card := map[string]string{"gtin": "04870712009990", "name": "Носки"}
	expect(t, e.do("POST", "/emu/api/nkt/cards", "", map[string]string{"gtin": "04870712009991", "name": "Носки"}), 400)
	expect(t, e.do("POST", "/emu/api/nkt/cards", "", map[string]string{"gtin": "04870712009990"}), 400)
	expect(t, e.do("POST", "/emu/api/nkt/cards", "", card), 201)
	expect(t, e.do("POST", "/emu/api/nkt/cards", "", card), 409)
	expect(t, e.do("GET", BasePath+"/products/info?gtin=04870712009990", token, nil), 200)
	if len(e.order(token, "04870712009990", 1)) != 1 {
		t.Fatal("order for new card failed")
	}
}

func TestOrderFlow(t *testing.T) {
	e := newEnv(t)
	token := e.login()

	rec := e.do("POST", BasePath+"/cises/my", token, map[string]any{"gtin": gtinShirt, "quantity": 3})
	expect(t, rec, 200)
	id := decode[map[string]string](t, rec)["uuid"]

	rec = e.do("GET", BasePath+"/cises/orders/"+id+"/status", token, nil)
	expect(t, rec, 200)
	if s := decode[map[string]string](t, rec)["status"]; s != "IN_PROGRESS" {
		t.Fatalf("status %s", s)
	}
	expect(t, e.do("GET", BasePath+"/cises/orders/"+id+"/result", token, nil), 409)

	e.clock.Add(5 * time.Second)
	rec = e.do("GET", BasePath+"/cises/orders/"+id+"/status", token, nil)
	if s := decode[map[string]string](t, rec)["status"]; s != "SUCCESS" {
		t.Fatalf("status %s", s)
	}

	rec = e.do("GET", BasePath+"/cises/orders/"+id+"/result", token, nil)
	expect(t, rec, 200)
	if rec.Header().Get("Content-Type") != "application/zip" {
		t.Fatalf("content type %s", rec.Header().Get("Content-Type"))
	}
	zr, err := zip.NewReader(bytes.NewReader(rec.Body.Bytes()), int64(rec.Body.Len()))
	if err != nil || len(zr.File) != 1 || zr.File[0].Name != id+".json" {
		t.Fatalf("zip: %v", err)
	}
	f, _ := zr.File[0].Open()
	raw, _ := io.ReadAll(f)
	var records []store.CisRecord
	if err := json.Unmarshal(raw, &records); err != nil || len(records) != 3 {
		t.Fatalf("records %s %v", raw, err)
	}
	for _, r := range records {
		if r.Status != "EMITTED" || r.GTIN != gtinShirt || r.PackageType != "UNIT" {
			t.Fatalf("record %+v", r)
		}
		if !strings.HasPrefix(r.Cis, "(01)"+gtinShirt+"(21)") || len(r.Code) != 31 {
			t.Fatalf("cis %q code %q", r.Cis, r.Code)
		}
		if strings.Count(r.PrintCode, "\x1d") != 2 || !strings.HasPrefix(r.PrintCode, r.Code+"\x1d91") || len(r.PrintCode) != 85 {
			t.Fatalf("print code %q (%d)", r.PrintCode, len(r.PrintCode))
		}
	}

	rec = e.do("POST", BasePath+"/cises/my", token, map[string]any{"gtin": gtinShirt})
	expect(t, rec, 200)
	listID := decode[map[string]string](t, rec)["uuid"]
	e.clock.Add(6 * time.Second)
	rec = e.do("GET", BasePath+"/cises/orders/"+listID+"/result", token, nil, "Accept", "application/json")
	expect(t, rec, 200)
	if n := len(decode[[]store.CisRecord](t, rec)); n != 3 {
		t.Fatalf("list order returned %d", n)
	}

	expect(t, e.do("POST", BasePath+"/cises/my", token, map[string]any{"quantity": 1}), 400)
	expect(t, e.do("POST", BasePath+"/cises/my", token, map[string]any{"gtin": "04870712009990", "quantity": 1}), 404)
	expect(t, e.do("POST", BasePath+"/cises/my", token, map[string]any{"gtin": "04870712001018", "quantity": 1}), 400)
	expect(t, e.do("POST", BasePath+"/cises/my", token, map[string]any{"gtin": gtinShirt, "quantity": 0}), 400)
	expect(t, e.do("POST", BasePath+"/cises/my", token, map[string]any{"gtin": gtinShirt, "quantity": 10001}), 400)
	expect(t, e.do("POST", BasePath+"/cises/my", token, "{bad json"), 400)
	expect(t, e.do("GET", BasePath+"/cises/orders/00000000-0000-0000-0000-000000000000/status", token, nil), 404)
}

func TestDocumentsFlow(t *testing.T) {
	e := newEnv(t)
	token := e.login()
	codes := e.order(token, gtinShirt, 4)
	a, b, c, d := codes[0], codes[1], codes[2], codes[3]

	id := e.sendDoc(token, store.DocApplicationReport, a.PrintCode, b.PrintCode, d.HumanCode)
	doc := e.processedDoc(token, id)
	if doc["status"] != "CHECKED_OK" || doc["type"] != store.DocApplicationReport {
		t.Fatalf("application report: %v", doc)
	}
	st := e.statuses(token, a.Cis, c.Code, d.PrintCode)
	if st[a.Cis].CisInfo.Status != "APPLIED" || st[c.Code].CisInfo.Status != "EMITTED" || st[d.PrintCode].CisInfo.Status != "APPLIED" {
		t.Fatalf("statuses %+v", st)
	}
	if st[a.Cis].CisInfo.OwnerBIN != testBIN || st[a.Cis].CisInfo.ProductName == "" {
		t.Fatalf("info %+v", st[a.Cis].CisInfo)
	}

	id = e.sendDoc(token, store.DocIntroduceLocal, a.Code, b.Cis)
	doc = e.processedDoc(token, id)
	if doc["status"] != "CHECKED_OK" {
		t.Fatalf("introduction: %v", doc)
	}
	st = e.statuses(token, a.Code, b.Code)
	if st[a.Code].CisInfo.Status != "INTRODUCED" || st[a.Code].CisInfo.ProducedDate == nil {
		t.Fatalf("statuses %+v", st)
	}

	id = e.sendDoc(token, store.DocIntroduceLocal, c.Code, d.Code)
	doc = e.processedDoc(token, id)
	if doc["status"] != "CHECKED_NOT_OK" {
		t.Fatalf("expected failure: %v", doc)
	}
	errs := doc["cisErrors"].([]any)
	if len(errs) != 1 || errs[0].(map[string]any)["errorCode"] != "CIS_WRONG_STATUS" {
		t.Fatalf("errors %v", errs)
	}
	if e.statuses(token, d.Code)[d.Code].CisInfo.Status != "APPLIED" {
		t.Fatal("failed document must not change statuses")
	}

	broken := strings.Replace(c.PrintCode, "\x1d", "", 1)
	id = e.sendDoc(token, store.DocApplicationReport, broken)
	doc = e.processedDoc(token, id)
	first := doc["cisErrors"].([]any)[0].(map[string]any)
	if doc["status"] != "CHECKED_NOT_OK" || first["errorCode"] != "GS_LOST" || first["errorMessage"] != "потерян символ GS" {
		t.Fatalf("gs lost: %v", doc)
	}
	if info := e.do("POST", BasePath+"/cises/info", token, []string{broken}); !strings.Contains(info.Body.String(), "потерян символ GS") {
		t.Fatalf("cises/info gs lost: %s", info.Body.String())
	}

	tampered := c.PrintCode[:len(c.PrintCode)-2] + "AA"
	id = e.sendDoc(token, store.DocApplicationReport, tampered)
	doc = e.processedDoc(token, id)
	if doc["cisErrors"].([]any)[0].(map[string]any)["errorCode"] != "CRYPTO_MISMATCH" {
		t.Fatalf("expected crypto mismatch: %v", doc)
	}

	body := docPayload(c.Code)
	rec := e.do("POST", BasePath+"/documents/create", token, map[string]string{
		"type": store.DocApplicationReport, "document": base64.StdEncoding.EncodeToString(body), "signature": sign.Emulate(docPayload(d.Code), testBIN)})
	expect(t, rec, 422)
	rec = e.do("POST", BasePath+"/documents/create", token, map[string]string{
		"type": store.DocApplicationReport, "document": base64.StdEncoding.EncodeToString(body), "signature": sign.Emulate(body, "000000000000")})
	expect(t, rec, 422)
	rec = e.do("POST", BasePath+"/documents/create", token, map[string]string{
		"type": "UNKNOWN", "document": base64.StdEncoding.EncodeToString(body), "signature": sign.Emulate(body, testBIN)})
	expect(t, rec, 400)
	expect(t, e.do("GET", BasePath+"/documents/nope/info", token, nil), 404)

	body = docPayload(a.Code)
	rec = e.do("POST", BasePath+"/documents/writeOff", token, map[string]string{
		"document": base64.StdEncoding.EncodeToString(body), "signature": sign.Emulate(body, testBIN)})
	expect(t, rec, 200)
	id = decode[map[string]string](t, rec)["documentId"]
	e.clock.Add(3 * time.Second)
	rec = e.do("GET", BasePath+"/documents/info/"+id, token, nil)
	expect(t, rec, 200)
	if decode[map[string]any](t, rec)["status"] != "CHECKED_OK" || e.statuses(token, a.Code)[a.Code].CisInfo.Status != "WITHDRAWN" {
		t.Fatal("withdrawal failed")
	}

	rec = e.do("GET", BasePath+"/doc/listV2?limit=100", token, nil)
	expect(t, rec, 200)
	if decode[map[string]any](t, rec)["total"].(float64) != 6 {
		t.Fatalf("list %s", rec.Body.String())
	}
}

func TestFormalEndpoint(t *testing.T) {
	e := newEnv(t)
	token := e.login()
	codes := e.order(token, gtinHoodie, 1)
	body := docPayload(codes[0].PrintCode)
	for _, p := range []string{"/edo-api/outgoing-documents/formal", BasePath + "/edo-api/outgoing-documents/formal"} {
		form := "type=" + store.DocApplicationReport +
			"&content=" + urlEncode(base64.StdEncoding.EncodeToString(body)) +
			"&signature=" + urlEncode(sign.Emulate(body, testBIN))
		req := httptest.NewRequest("POST", p, strings.NewReader(form))
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		req.Header.Set("Authorization", "Bearer "+token)
		rec := httptest.NewRecorder()
		e.srv.ServeHTTP(rec, req)
		expect(t, rec, 200)
	}
	e.clock.Add(3 * time.Second)
	if e.statuses(token, codes[0].Code)[codes[0].Code].CisInfo.Status != "APPLIED" {
		t.Fatal("formal endpoint did not apply")
	}
}

func urlEncode(s string) string {
	return strings.NewReplacer("+", "%2B", "/", "%2F", "=", "%3D").Replace(s)
}

func TestAnnulment(t *testing.T) {
	e := newEnv(t)
	token := e.login()
	codes := e.order(token, gtinShirt, 2)
	a, b := codes[0], codes[1]
	id := e.sendDoc(token, store.DocApplicationReport, a.PrintCode)
	e.processedDoc(token, id)

	rec := e.do("POST", "/emu/api/time", "", map[string]any{"addDays": 29})
	expect(t, rec, 200)
	if e.statuses(token, b.Code)[b.Code].CisInfo.Status != "EMITTED" {
		t.Fatal("annulled too early")
	}
	expect(t, e.do("POST", "/emu/api/time", "", map[string]any{"addDays": 1}), 200)
	st := e.statuses(token, a.Code, b.Code)
	if st[b.Code].CisInfo.Status != "WRITTEN_OFF" || st[b.Code].CisInfo.StatusExt != "ANNULLED" {
		t.Fatalf("not annulled: %+v", st[b.Code])
	}
	if st[a.Code].CisInfo.Status != "APPLIED" {
		t.Fatal("applied code must not be annulled")
	}
	expect(t, e.do("POST", "/emu/api/time", "", map[string]any{"reset": true}), 200)
	if e.statuses(token, b.Code)[b.Code].CisInfo.Status != "WRITTEN_OFF" {
		t.Fatal("annulment must be permanent")
	}
	id = e.sendDoc(token, store.DocApplicationReport, b.PrintCode)
	doc := e.processedDoc(token, id)
	if doc["cisErrors"].([]any)[0].(map[string]any)["errorCode"] != "CIS_ANNULLED" {
		t.Fatalf("doc %v", doc)
	}
}

func TestPersistenceAndReset(t *testing.T) {
	e := newEnv(t)
	token := e.login()
	codes := e.order(token, gtinShirt, 2)
	expect(t, e.do("POST", "/emu/api/nkt/cards", "", map[string]string{"gtin": "04870712009990", "name": "Носки"}), 201)

	e.open()
	if e.statuses(token, codes[0].Code)[codes[0].Code].CisInfo.Status != "EMITTED" {
		t.Fatal("state lost after restart")
	}
	if _, ok := e.store.Product("04870712009990"); !ok {
		t.Fatal("card lost after restart")
	}

	rec := e.do("POST", "/emu/reset", "", "", "Content-Type", "application/x-www-form-urlencoded", "Accept", "text/html")
	expect(t, rec, http.StatusSeeOther)
	if _, ok := e.store.Product("04870712009990"); ok {
		t.Fatal("reset kept added card")
	}
	expect(t, e.do("POST", BasePath+"/cises/info", token, []string{codes[0].Code}), 404)
	snap := e.store.Snapshot()
	if snap.TotalCodes != 0 || len(snap.Products) != 3 || len(snap.Orders) != 0 {
		t.Fatalf("snapshot after reset %+v", snap)
	}
}

func TestConsoleAndService(t *testing.T) {
	e := newEnv(t)
	token := e.login()
	e.order(token, gtinShirt, 1)

	rec := e.do("GET", "/emu/", "", nil)
	expect(t, rec, 200)
	html := rec.Body.String()
	for _, want := range []string{"Эмулятор ИС МПТ", "Кендір Тігін", gtinShirt, "Сбросить эмулятор", "/api/v3/true-api/cises/my"} {
		if !strings.Contains(html, want) {
			t.Errorf("console lacks %q", want)
		}
	}
	if strings.Contains(html, "\u2014") {
		t.Error("console contains long dash")
	}
	rec = e.do("GET", "/emu/api/state", "", nil)
	expect(t, rec, 200)
	if !strings.Contains(rec.Body.String(), `"totalCodes":1`) {
		t.Fatalf("state %s", rec.Body.String())
	}
	expect(t, e.do("GET", "/healthz", "", nil), 200)
	rec = e.do("GET", BasePath+"/unknown", token, nil)
	expect(t, rec, 404)
	if !strings.Contains(rec.Body.String(), "Метод с указанным URL не найден") {
		t.Fatal(rec.Body.String())
	}
	expect(t, e.do("POST", "/emu/api/time", "", "days=7", "Content-Type", "application/x-www-form-urlencoded", "Accept", "text/html"), http.StatusSeeOther)
	if e.store.Offset() != 7*24*time.Hour {
		t.Fatal("offset not applied")
	}
}
