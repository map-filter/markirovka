package agent

import "time"

type Backoff struct {
	Min     time.Duration
	Max     time.Duration
	current time.Duration
}

func (b *Backoff) Next() time.Duration {
	if b.current == 0 {
		b.current = b.Min
	} else {
		b.current *= 2
	}
	if b.current > b.Max {
		b.current = b.Max
	}
	return b.current
}

func (b *Backoff) Reset() {
	b.current = 0
}
