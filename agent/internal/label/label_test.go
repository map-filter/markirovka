package label

import (
	"bytes"
	"testing"

	"markirovka/agent/internal/printer"
)

func TestTestZPLIsValid(t *testing.T) {
	data := TestZPL()
	if err := printer.CheckZPL(data); err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{"^BXN,5,200", "^FD~101" + TestGTIN, "~193"} {
		if !bytes.Contains(data, []byte(want)) {
			t.Errorf("label does not contain %q", want)
		}
	}
}

func TestTestGTINCheckDigit(t *testing.T) {
	sum := 0
	for i := 0; i < len(TestGTIN)-1; i++ {
		d := int(TestGTIN[i] - '0')
		if (len(TestGTIN)-1-i)%2 == 1 {
			d *= 3
		}
		sum += d
	}
	want := (10 - sum%10) % 10
	if got := int(TestGTIN[len(TestGTIN)-1] - '0'); got != want {
		t.Fatalf("check digit %d, want %d", got, want)
	}
}
