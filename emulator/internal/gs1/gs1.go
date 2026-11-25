package gs1

import (
	"crypto/rand"
	"errors"
	"fmt"
	"math/big"
	"strings"
)

const (
	GS = "\x1d"

	SerialLength = 13
	KeyLength    = 4
	CryptoLength = 44

	ai82Charset   = "!\"%&'()*+,-./0123456789:;<=>?ABCDEFGHIJKLMNOPQRSTUVWXYZ_abcdefghijklmnopqrstuvwxyz"
	SerialCharset = "!%&*+,-./0123456789:;<=>?ABCDEFGHIJKLMNOPQRSTUVWXYZ_abcdefghijklmnopqrstuvwxyz"
	KeyCharset    = "0123456789ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz"
	CryptoCharset = "ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz0123456789+/="

	maxSerialLength = 20
)

var (
	ErrGSLost       = errors.New("потерян символ GS")
	ErrFormat       = errors.New("неверный формат КМ")
	ErrGTINCheck    = errors.New("неверная контрольная цифра GTIN")
	ErrSerialFormat = errors.New("недопустимый символ в серийном номере")
)

type Code struct {
	GTIN   string
	Serial string
	Key    string
	Crypto string
}

func (c Code) HasCrypto() bool {
	return c.Key != "" || c.Crypto != ""
}

func (c Code) KI() string {
	return "01" + c.GTIN + "21" + c.Serial
}

func (c Code) CIS() string {
	return "(01)" + c.GTIN + "(21)" + c.Serial
}

func (c Code) Raw() string {
	s := c.KI()
	if c.Key != "" {
		s += GS + "91" + c.Key
	}
	if c.Crypto != "" {
		s += GS + "92" + c.Crypto
	}
	return s
}

func (c Code) HumanReadable() string {
	s := c.CIS()
	if c.Key != "" {
		s += "(91)" + c.Key
	}
	if c.Crypto != "" {
		s += "(92)" + c.Crypto
	}
	return s
}

func CheckDigit(body string) (int, error) {
	if body == "" {
		return 0, ErrFormat
	}
	sum := 0
	for i := len(body) - 1; i >= 0; i-- {
		d := body[i]
		if d < '0' || d > '9' {
			return 0, ErrFormat
		}
		n := int(d - '0')
		if (len(body)-1-i)%2 == 0 {
			n *= 3
		}
		sum += n
	}
	return (10 - sum%10) % 10, nil
}

func ValidateGTIN(gtin string) error {
	if len(gtin) != 14 {
		return fmt.Errorf("GTIN должен состоять из 14 цифр: %q", gtin)
	}
	want, err := CheckDigit(gtin[:13])
	if err != nil {
		return fmt.Errorf("GTIN должен состоять из 14 цифр: %q", gtin)
	}
	if int(gtin[13]-'0') != want {
		return ErrGTINCheck
	}
	return nil
}

func Build(gtin, serial, key, crypto string) (Code, error) {
	if err := ValidateGTIN(gtin); err != nil {
		return Code{}, err
	}
	if len(serial) != SerialLength || !onlyFrom(serial, ai82Charset) {
		return Code{}, ErrSerialFormat
	}
	if len(key) != KeyLength || !onlyFrom(key, KeyCharset) {
		return Code{}, fmt.Errorf("ключ проверки (91) должен состоять из %d символов", KeyLength)
	}
	if len(crypto) != CryptoLength || !onlyFrom(crypto, CryptoCharset) {
		return Code{}, fmt.Errorf("криптоподпись (92) должна состоять из %d символов", CryptoLength)
	}
	return Code{GTIN: gtin, Serial: serial, Key: key, Crypto: crypto}, nil
}

func Parse(input string) (Code, error) {
	s := strings.TrimSpace(input)
	s = strings.TrimPrefix(s, "]d2")
	s = strings.TrimPrefix(s, "]C1")
	s = strings.TrimLeft(s, GS)
	if strings.HasPrefix(s, "(") {
		return parseHuman(s)
	}
	return parseRaw(s)
}

func parseRaw(s string) (Code, error) {
	if len(s) < 2+14+2+1 || !strings.HasPrefix(s, "01") {
		return Code{}, ErrFormat
	}
	gtin := s[2:16]
	if err := ValidateGTIN(gtin); err != nil {
		return Code{}, err
	}
	if s[16:18] != "21" {
		return Code{}, ErrFormat
	}
	rest := s[18:]
	parts := strings.Split(rest, GS)
	serial := parts[0]
	if len(serial) > SerialLength && serial[SerialLength:min(len(serial), SerialLength+2)] == "91" {
		return Code{}, ErrGSLost
	}
	if len(serial) > maxSerialLength {
		return Code{}, ErrFormat
	}
	if serial == "" || !onlyFrom(serial, ai82Charset) {
		return Code{}, ErrSerialFormat
	}
	c := Code{GTIN: gtin, Serial: serial}
	for _, p := range parts[1:] {
		switch {
		case strings.HasPrefix(p, "91"):
			v := p[2:]
			if len(v) > KeyLength && strings.HasPrefix(v[KeyLength:], "92") {
				return Code{}, ErrGSLost
			}
			c.Key = v
		case strings.HasPrefix(p, "92"):
			c.Crypto = p[2:]
		default:
			return Code{}, ErrFormat
		}
	}
	return c, nil
}

func parseHuman(s string) (Code, error) {
	var c Code
	seen := map[string]bool{}
	for s != "" {
		if len(s) < 4 || s[0] != '(' || s[3] != ')' {
			return Code{}, ErrFormat
		}
		ai := s[1:3]
		s = s[4:]
		end := strings.Index(s, "(")
		for end >= 0 && !isAIStart(s[end:]) {
			next := strings.Index(s[end+1:], "(")
			if next < 0 {
				end = -1
				break
			}
			end += next + 1
		}
		val := s
		if end >= 0 {
			val, s = s[:end], s[end:]
		} else {
			s = ""
		}
		if seen[ai] {
			return Code{}, ErrFormat
		}
		seen[ai] = true
		switch ai {
		case "01":
			c.GTIN = val
		case "21":
			c.Serial = val
		case "91":
			c.Key = val
		case "92":
			c.Crypto = val
		default:
			return Code{}, ErrFormat
		}
	}
	if err := ValidateGTIN(c.GTIN); err != nil {
		return Code{}, err
	}
	if c.Serial == "" || len(c.Serial) > maxSerialLength || !onlyFrom(c.Serial, ai82Charset) {
		return Code{}, ErrSerialFormat
	}
	return c, nil
}

func isAIStart(s string) bool {
	if len(s) < 4 || s[3] != ')' {
		return false
	}
	switch s[1:3] {
	case "01", "21", "91", "92":
		return true
	}
	return false
}

func RandomString(charset string, n int) string {
	max := big.NewInt(int64(len(charset)))
	b := make([]byte, n)
	for i := range b {
		v, err := rand.Int(rand.Reader, max)
		if err != nil {
			panic(err)
		}
		b[i] = charset[v.Int64()]
	}
	return string(b)
}

func RandomSerial() string {
	return RandomString(SerialCharset, SerialLength)
}

func onlyFrom(s, charset string) bool {
	for i := 0; i < len(s); i++ {
		if strings.IndexByte(charset, s[i]) < 0 {
			return false
		}
	}
	return true
}
