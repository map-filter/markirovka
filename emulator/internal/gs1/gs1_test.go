package gs1

import (
	"errors"
	"strings"
	"testing"
)

const (
	testGTIN   = "04870712001017"
	testSerial = "Ab1-cD2.eF3:g"
	testKey    = "K9z0"
	testCrypto = "QUJDREVGR0hJSktMTU5PUFFSU1RVVldYWVphYmNkZWY="
)

func TestCheckDigit(t *testing.T) {
	cases := map[string]int{
		"0487071200101": 7,
		"0487071200102": 4,
		"0000004621065": 4,
	}
	for body, want := range cases {
		got, err := CheckDigit(body)
		if err != nil || got != want {
			t.Errorf("CheckDigit(%s) = %d, %v; want %d", body, got, err, want)
		}
	}
	if _, err := CheckDigit("12a"); err == nil {
		t.Error("expected error for non-digit body")
	}
}

func TestValidateGTIN(t *testing.T) {
	if err := ValidateGTIN(testGTIN); err != nil {
		t.Fatalf("valid GTIN rejected: %v", err)
	}
	if err := ValidateGTIN("04870712001018"); !errors.Is(err, ErrGTINCheck) {
		t.Fatalf("expected check digit error, got %v", err)
	}
	for _, bad := range []string{"", "4870712001017", "0487071200101A", "048707120010170"} {
		if err := ValidateGTIN(bad); err == nil {
			t.Errorf("expected error for %q", bad)
		}
	}
}

func TestBuildAndForms(t *testing.T) {
	c, err := Build(testGTIN, testSerial, testKey, testCrypto)
	if err != nil {
		t.Fatal(err)
	}
	wantRaw := "01" + testGTIN + "21" + testSerial + "\x1d91" + testKey + "\x1d92" + testCrypto
	if c.Raw() != wantRaw {
		t.Fatalf("raw = %q", c.Raw())
	}
	if got := c.HumanReadable(); got != "(01)"+testGTIN+"(21)"+testSerial+"(91)"+testKey+"(92)"+testCrypto {
		t.Fatalf("human = %q", got)
	}
	if c.CIS() != "(01)"+testGTIN+"(21)"+testSerial || c.KI() != "01"+testGTIN+"21"+testSerial {
		t.Fatalf("cis/ki = %q %q", c.CIS(), c.KI())
	}
}

func TestBuildRejects(t *testing.T) {
	if _, err := Build("04870712001018", testSerial, testKey, testCrypto); err == nil {
		t.Error("bad GTIN accepted")
	}
	if _, err := Build(testGTIN, "short", testKey, testCrypto); err == nil {
		t.Error("short serial accepted")
	}
	if _, err := Build(testGTIN, testSerial, "K9", testCrypto); err == nil {
		t.Error("short key accepted")
	}
	if _, err := Build(testGTIN, testSerial, testKey, "abc"); err == nil {
		t.Error("short crypto accepted")
	}
}

func TestParseRaw(t *testing.T) {
	c, _ := Build(testGTIN, testSerial, testKey, testCrypto)
	for _, in := range []string{c.Raw(), "]d2" + c.Raw(), "\x1d" + c.Raw()} {
		got, err := Parse(in)
		if err != nil {
			t.Fatalf("Parse(%q): %v", in, err)
		}
		if got != c {
			t.Fatalf("Parse = %+v, want %+v", got, c)
		}
	}
}

func TestParseKIOnly(t *testing.T) {
	got, err := Parse("01" + testGTIN + "21" + testSerial)
	if err != nil {
		t.Fatal(err)
	}
	if got.GTIN != testGTIN || got.Serial != testSerial || got.HasCrypto() {
		t.Fatalf("got %+v", got)
	}
}

func TestParseHuman(t *testing.T) {
	c, _ := Build(testGTIN, testSerial, testKey, testCrypto)
	got, err := Parse(c.HumanReadable())
	if err != nil {
		t.Fatal(err)
	}
	if got != c {
		t.Fatalf("got %+v", got)
	}
	ki, err := Parse(c.CIS())
	if err != nil || ki.KI() != c.KI() {
		t.Fatalf("cis parse: %+v %v", ki, err)
	}
}

func TestParseGSLost(t *testing.T) {
	c, _ := Build(testGTIN, testSerial, testKey, testCrypto)
	noGS := strings.ReplaceAll(c.Raw(), GS, "")
	firstLost := strings.Replace(c.Raw(), GS, "", 1)
	secondLost := c.KI() + GS + "91" + testKey + "92" + testCrypto
	for name, in := range map[string]string{"all": noGS, "first": firstLost, "second": secondLost} {
		_, err := Parse(in)
		if !errors.Is(err, ErrGSLost) {
			t.Errorf("%s: expected ErrGSLost, got %v", name, err)
		}
		if err != nil && err.Error() != "потерян символ GS" {
			t.Errorf("%s: message %q", name, err.Error())
		}
	}
}

func TestParseInvalid(t *testing.T) {
	for _, in := range []string{"", "garbage", "02" + testGTIN + "21abc", "01" + "04870712001018" + "21" + testSerial, "01" + testGTIN + "22" + testSerial, "01" + testGTIN + "21" + strings.Repeat("a", 21)} {
		if _, err := Parse(in); err == nil {
			t.Errorf("expected error for %q", in)
		}
	}
}

func TestRandomSerial(t *testing.T) {
	seen := map[string]bool{}
	for i := 0; i < 2000; i++ {
		s := RandomSerial()
		if len(s) != SerialLength {
			t.Fatalf("len %d", len(s))
		}
		if strings.ContainsAny(s, "\"'\\^~()") {
			t.Fatalf("unsafe char in %q", s)
		}
		if seen[s] {
			t.Fatalf("duplicate %q", s)
		}
		seen[s] = true
		if _, err := Build(testGTIN, s, testKey, testCrypto); err != nil {
			t.Fatalf("serial %q rejected: %v", s, err)
		}
	}
}
