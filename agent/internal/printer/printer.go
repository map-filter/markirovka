package printer

import (
	"context"
	"errors"
	"fmt"
	"net/url"
	"strings"
)

const (
	FormatZPL = "zpl"
	FormatPDF = "pdf"
	FormatRaw = "raw"
)

type Job struct {
	ID     string
	Format string
	Copies int
	Data   []byte
}

type Printer interface {
	Print(ctx context.Context, job Job) error
	URI() string
}

func Open(uri string) (Printer, error) {
	if strings.TrimSpace(uri) == "" {
		return nil, errors.New("printer uri is empty")
	}
	u, err := url.Parse(uri)
	if err != nil {
		return nil, fmt.Errorf("parse printer uri %q: %w", uri, err)
	}
	switch u.Scheme {
	case "tcp":
		return newTCP(u)
	case "file":
		return newFile(u)
	case "cups":
		return newCUPS(u)
	default:
		return nil, fmt.Errorf("unsupported printer uri scheme %q (use tcp://, file:// or cups://)", u.Scheme)
	}
}

func Validate(job Job) error {
	switch job.Format {
	case FormatZPL:
		return CheckZPL(job.Data)
	case FormatPDF, FormatRaw:
	default:
		return fmt.Errorf("unsupported job format %q", job.Format)
	}
	if len(job.Data) == 0 {
		return errors.New("job payload is empty")
	}
	return nil
}

func copies(job Job) int {
	if job.Copies < 1 {
		return 1
	}
	return job.Copies
}
