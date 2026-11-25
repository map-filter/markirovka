package printer

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"net/url"
	"os/exec"
	"regexp"
	"strconv"
	"strings"
)

var queueName = regexp.MustCompile(`^[A-Za-z0-9_][A-Za-z0-9_.@-]*$`)

type CUPS struct {
	queue   string
	Command string
}

func newCUPS(u *url.URL) (*CUPS, error) {
	queue := u.Host
	if queue == "" {
		queue = strings.Trim(u.Opaque+u.Path, "/")
	}
	if !queueName.MatchString(queue) {
		return nil, fmt.Errorf("invalid cups queue name %q", queue)
	}
	return &CUPS{queue: queue, Command: "lp"}, nil
}

func (p *CUPS) URI() string {
	return "cups://" + p.queue
}

func (p *CUPS) Print(ctx context.Context, job Job) error {
	path, err := exec.LookPath(p.Command)
	if err != nil {
		return fmt.Errorf("cups printing needs the %q command (install cups-client or use tcp://): %w", p.Command, err)
	}
	args := []string{"-d", p.queue, "-o", "raw", "-n", strconv.Itoa(copies(job))}
	if job.ID != "" {
		args = append(args, "-t", safeName(job.ID))
	}
	cmd := exec.CommandContext(ctx, path, args...)
	cmd.Stdin = bytes.NewReader(job.Data)
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		msg := strings.TrimSpace(stderr.String())
		var exitErr *exec.ExitError
		if errors.As(err, &exitErr) && msg != "" {
			return fmt.Errorf("lp failed: %s", msg)
		}
		return fmt.Errorf("lp failed: %w", err)
	}
	return nil
}
