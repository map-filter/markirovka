package sign

import (
	"bytes"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"strings"
)

const Magic = "MKEMU-CMS-v1"

var (
	ErrFormat         = errors.New("неизвестный формат подписи")
	ErrEmulatedOff    = errors.New("эмулированная подпись отключена")
	ErrRealCMSOff     = errors.New("настоящая подпись CMS не принимается эмулятором (EMU_ACCEPT_ANY_CMS=false)")
	ErrDigestMismatch = errors.New("подпись не прошла проверку")
)

type Options struct {
	AcceptEmulated bool
	AcceptAnyCMS   bool
}

type Result struct {
	Emulated bool
	BIN      string
	Data     []byte
}

func Digest(data []byte, bin string) string {
	h := sha256.New()
	h.Write(data)
	h.Write([]byte(bin))
	return hex.EncodeToString(h.Sum(nil))
}

func Emulate(data []byte, bin string) string {
	text := strings.Join([]string{
		Magic,
		base64.StdEncoding.EncodeToString(data),
		"BIN:" + bin,
		"SIG:" + Digest(data, bin),
	}, "\n")
	return base64.StdEncoding.EncodeToString([]byte(text))
}

func Verify(cms string, opts Options) (Result, error) {
	raw, err := DecodeBase64(cms)
	if err != nil || len(raw) == 0 {
		return Result{}, ErrFormat
	}
	if bytes.HasPrefix(raw, []byte(Magic)) {
		if !opts.AcceptEmulated {
			return Result{}, ErrEmulatedOff
		}
		return verifyEmulated(string(raw))
	}
	if raw[0] == 0x30 {
		if !opts.AcceptAnyCMS {
			return Result{}, ErrRealCMSOff
		}
		return Result{}, nil
	}
	return Result{}, ErrFormat
}

func verifyEmulated(text string) (Result, error) {
	lines := strings.Split(strings.ReplaceAll(strings.TrimRight(text, "\r\n"), "\r\n", "\n"), "\n")
	if len(lines) != 4 || lines[0] != Magic || !strings.HasPrefix(lines[2], "BIN:") || !strings.HasPrefix(lines[3], "SIG:") {
		return Result{}, ErrFormat
	}
	data, err := DecodeBase64(lines[1])
	if err != nil {
		return Result{}, ErrFormat
	}
	bin := strings.TrimSpace(strings.TrimPrefix(lines[2], "BIN:"))
	sig := strings.TrimSpace(strings.TrimPrefix(lines[3], "SIG:"))
	if bin == "" || !strings.EqualFold(sig, Digest(data, bin)) {
		return Result{}, ErrDigestMismatch
	}
	return Result{Emulated: true, BIN: bin, Data: data}, nil
}

func DecodeBase64(s string) ([]byte, error) {
	s = strings.Map(func(r rune) rune {
		if r == '\n' || r == '\r' || r == ' ' || r == '\t' {
			return -1
		}
		return r
	}, s)
	if b, err := base64.StdEncoding.DecodeString(s); err == nil {
		return b, nil
	}
	return base64.RawStdEncoding.DecodeString(strings.TrimRight(s, "="))
}
