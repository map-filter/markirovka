package printer

import (
	"bytes"
	"errors"
)

func CheckZPL(data []byte) error {
	trimmed := bytes.TrimLeft(data, " \t\r\n")
	if !bytes.HasPrefix(trimmed, []byte("^XA")) {
		return errors.New("zpl payload must start with ^XA")
	}
	if !bytes.Contains(trimmed, []byte("^XZ")) {
		return errors.New("zpl payload must contain ^XZ")
	}
	return nil
}
