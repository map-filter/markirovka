package printer

import (
	"context"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"strings"
)

type File struct {
	dir string
}

func newFile(u *url.URL) (*File, error) {
	dir := u.Path
	if u.Host != "" && u.Host != "localhost" {
		dir = u.Host + u.Path
	}
	if len(dir) >= 3 && dir[0] == '/' && dir[2] == ':' {
		dir = dir[1:]
	}
	if dir == "" {
		return nil, fmt.Errorf("file printer uri has no path: %q", u.String())
	}
	return &File{dir: filepath.FromSlash(dir)}, nil
}

func (p *File) URI() string {
	return "file://" + filepath.ToSlash(p.dir)
}

func (p *File) Dir() string {
	return p.dir
}

func (p *File) Print(ctx context.Context, job Job) error {
	if err := os.MkdirAll(p.dir, 0o755); err != nil {
		return fmt.Errorf("create output dir: %w", err)
	}
	base := safeName(job.ID)
	ext := extension(job.Format)
	for n := 1; n <= copies(job); n++ {
		if err := ctx.Err(); err != nil {
			return err
		}
		name := filepath.Join(p.dir, fmt.Sprintf("%s-%d.%s", base, n, ext))
		if err := writeAtomic(name, job.Data); err != nil {
			return err
		}
	}
	return nil
}

func writeAtomic(name string, data []byte) error {
	tmp, err := os.CreateTemp(filepath.Dir(name), ".tmp-*")
	if err != nil {
		return fmt.Errorf("create temp file: %w", err)
	}
	defer os.Remove(tmp.Name())
	if _, err := tmp.Write(data); err != nil {
		tmp.Close()
		return fmt.Errorf("write %s: %w", name, err)
	}
	if err := tmp.Close(); err != nil {
		return fmt.Errorf("close %s: %w", name, err)
	}
	if err := os.Rename(tmp.Name(), name); err != nil {
		return fmt.Errorf("rename to %s: %w", name, err)
	}
	return nil
}

func extension(format string) string {
	switch format {
	case FormatZPL:
		return "zpl"
	case FormatPDF:
		return "pdf"
	default:
		return "bin"
	}
}

func safeName(id string) string {
	name := strings.Map(func(r rune) rune {
		switch {
		case r >= 'a' && r <= 'z', r >= 'A' && r <= 'Z', r >= '0' && r <= '9', r == '-', r == '_':
			return r
		default:
			return '_'
		}
	}, id)
	if name == "" {
		return "job"
	}
	return name
}
