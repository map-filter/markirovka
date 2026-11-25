package sign

import (
	"encoding/base64"
	"errors"
	"testing"
)

var on = Options{AcceptEmulated: true}

func TestEmulatedRoundTrip(t *testing.T) {
	cms := Emulate([]byte("PIQKFTTUVWQQGDCOLVVTOPHOOBDCPE"), "190440015237")
	res, err := Verify(cms, on)
	if err != nil {
		t.Fatal(err)
	}
	if !res.Emulated || res.BIN != "190440015237" || string(res.Data) != "PIQKFTTUVWQQGDCOLVVTOPHOOBDCPE" {
		t.Fatalf("got %+v", res)
	}
}

func TestEmulatedTampered(t *testing.T) {
	text := Magic + "\n" + base64.StdEncoding.EncodeToString([]byte("abc")) + "\nBIN:190440015237\nSIG:00"
	_, err := Verify(base64.StdEncoding.EncodeToString([]byte(text)), on)
	if !errors.Is(err, ErrDigestMismatch) {
		t.Fatalf("got %v", err)
	}
}

func TestEmulatedDisabled(t *testing.T) {
	_, err := Verify(Emulate([]byte("x"), "1"), Options{})
	if !errors.Is(err, ErrEmulatedOff) {
		t.Fatalf("got %v", err)
	}
}

func TestRealCMS(t *testing.T) {
	der := base64.StdEncoding.EncodeToString([]byte{0x30, 0x82, 0x01, 0x00})
	if _, err := Verify(der, on); !errors.Is(err, ErrRealCMSOff) {
		t.Fatalf("got %v", err)
	}
	res, err := Verify(der, Options{AcceptAnyCMS: true})
	if err != nil || res.Emulated {
		t.Fatalf("got %+v %v", res, err)
	}
}

func TestGarbage(t *testing.T) {
	for _, in := range []string{"", "!!!", base64.StdEncoding.EncodeToString([]byte("hello"))} {
		if _, err := Verify(in, Options{AcceptEmulated: true, AcceptAnyCMS: true}); err == nil {
			t.Errorf("accepted %q", in)
		}
	}
}
