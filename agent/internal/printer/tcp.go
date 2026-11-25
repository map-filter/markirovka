package printer

import (
	"context"
	"fmt"
	"net"
	"net/url"
	"time"
)

type TCP struct {
	addr         string
	DialTimeout  time.Duration
	WriteTimeout time.Duration
}

func newTCP(u *url.URL) (*TCP, error) {
	if u.Hostname() == "" {
		return nil, fmt.Errorf("tcp printer uri has no host: %q", u.String())
	}
	port := u.Port()
	if port == "" {
		port = "9100"
	}
	return &TCP{
		addr:         net.JoinHostPort(u.Hostname(), port),
		DialTimeout:  5 * time.Second,
		WriteTimeout: 30 * time.Second,
	}, nil
}

func (p *TCP) URI() string {
	return "tcp://" + p.addr
}

func (p *TCP) Print(ctx context.Context, job Job) error {
	for i := 0; i < copies(job); i++ {
		if err := p.send(ctx, job.Data); err != nil {
			return fmt.Errorf("copy %d: %w", i+1, err)
		}
	}
	return nil
}

func (p *TCP) send(ctx context.Context, data []byte) error {
	dialer := net.Dialer{Timeout: p.DialTimeout}
	conn, err := dialer.DialContext(ctx, "tcp", p.addr)
	if err != nil {
		return fmt.Errorf("connect to printer %s: %w", p.addr, err)
	}
	defer conn.Close()
	if err := conn.SetWriteDeadline(time.Now().Add(p.WriteTimeout)); err != nil {
		return err
	}
	if _, err := conn.Write(data); err != nil {
		return fmt.Errorf("write to printer %s: %w", p.addr, err)
	}
	if tc, ok := conn.(*net.TCPConn); ok {
		if err := tc.CloseWrite(); err != nil {
			return fmt.Errorf("finish write to printer %s: %w", p.addr, err)
		}
	}
	return nil
}
