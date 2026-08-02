package main

import (
	"context"
	"crypto/tls"
	"log/slog"
	"net/http"
	"time"

	"github.com/coder/websocket"

	"github.com/denisakp/ogoune/pkg/agentwire"
)

// wsConn is the minimal socket surface the streamer needs, so tests can supply a
// fake without a real network.
type wsConn interface {
	Write(ctx context.Context, data []byte) error
	Close(code websocket.StatusCode, reason string) error
}

// dialFunc establishes a connection, returning the HTTP status of the upgrade
// attempt (used to distinguish a 401 auth rejection from a transient failure).
type dialFunc func(ctx context.Context) (wsConn, int, error)

// Streamer owns the resilient send loop: connect, stream one frame per interval,
// and reconnect with back-off. A permanent 401 uses a slow capped back-off
// (retrying forever so it self-heals on re-validation); transient failures use a
// fast capped back-off.
type Streamer struct {
	cfg       Config
	collector Collector
	dial      dialFunc
	sleep     func(ctx context.Context, d time.Duration) bool
	transient backoff
	auth      backoff
}

// NewStreamer wires the production dialer and clock.
func NewStreamer(cfg Config, collector Collector) *Streamer {
	return &Streamer{
		cfg:       cfg,
		collector: collector,
		dial:      realDial(cfg),
		sleep:     sleepCtx,
		transient: backoff{base: 1 * time.Second, max: 30 * time.Second},
		auth:      backoff{base: 30 * time.Second, max: 5 * time.Minute},
	}
}

// Run connects and streams until ctx is cancelled, reconnecting on failure. It
// only returns when ctx is done.
func (s *Streamer) Run(ctx context.Context) error {
	for {
		if ctx.Err() != nil {
			return ctx.Err()
		}
		conn, status, err := s.dial(ctx)
		if err != nil {
			if ctx.Err() != nil {
				return ctx.Err()
			}
			var d time.Duration
			if status == http.StatusUnauthorized {
				d = s.auth.next()
				slog.Warn("agent: credential rejected — backing off (will self-heal if re-validated)", "retry_in", d.String(), "status", status)
			} else {
				d = s.transient.next()
				slog.Warn("agent: connect failed — retrying", "retry_in", d.String(), "error", err)
			}
			if !s.sleep(ctx, d) {
				return ctx.Err()
			}
			continue
		}
		// Connected: reset both regimes.
		s.transient.reset()
		s.auth.reset()
		slog.Info("agent: connected", "url", s.cfg.BackendURL)

		streamErr := s.streamLoop(ctx, conn)
		_ = conn.Close(websocket.StatusNormalClosure, "")
		if ctx.Err() != nil {
			return ctx.Err()
		}
		d := s.transient.next()
		slog.Warn("agent: stream ended — reconnecting", "retry_in", d.String(), "error", streamErr)
		if !s.sleep(ctx, d) {
			return ctx.Err()
		}
	}
}

// streamLoop sends one frame immediately, then one per interval, until an error
// or ctx cancellation.
func (s *Streamer) streamLoop(ctx context.Context, conn wsConn) error {
	send := func() error {
		frame, err := s.collector.Collect(ctx)
		if err != nil {
			return err
		}
		b, err := agentwire.Encode(frame)
		if err != nil {
			return err
		}
		if err := conn.Write(ctx, b); err != nil {
			return err
		}
		slog.Debug("agent: frame sent",
			"cpu_pct", frame.CPUPct, "mem_pct", frame.MemPct, "disks", len(frame.Disks))
		return nil
	}

	if err := send(); err != nil {
		return err
	}
	ticker := time.NewTicker(s.cfg.Interval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-ticker.C:
			if err := send(); err != nil {
				return err
			}
		}
	}
}

// backoff is an exponential back-off with a cap. next() returns the current
// delay and advances; reset() returns to the base delay.
type backoff struct {
	base, max, cur time.Duration
}

func (b *backoff) next() time.Duration {
	d := b.cur
	if d == 0 {
		d = b.base
	}
	nx := d * 2
	if nx > b.max {
		nx = b.max
	}
	b.cur = nx
	return d
}

func (b *backoff) reset() { b.cur = 0 }

// sleepCtx sleeps for d, returning false if ctx is cancelled first.
func sleepCtx(ctx context.Context, d time.Duration) bool {
	t := time.NewTimer(d)
	defer t.Stop()
	select {
	case <-ctx.Done():
		return false
	case <-t.C:
		return true
	}
}

// realDial dials the backend with the host bearer credential.
func realDial(cfg Config) dialFunc {
	var httpClient *http.Client
	if cfg.InsecureSkipVerify {
		httpClient = &http.Client{Transport: &http.Transport{TLSClientConfig: &tls.Config{InsecureSkipVerify: true}}} //nolint:gosec // opt-in dev flag
	}
	return func(ctx context.Context) (wsConn, int, error) {
		opts := &websocket.DialOptions{
			HTTPHeader: http.Header{"Authorization": []string{"Bearer " + cfg.Credential}},
		}
		if httpClient != nil {
			opts.HTTPClient = httpClient
		}
		c, resp, err := websocket.Dial(ctx, cfg.BackendURL, opts)
		status := 0
		if resp != nil {
			status = resp.StatusCode
		}
		if err != nil {
			return nil, status, err
		}
		return &coderConn{c: c}, status, nil
	}
}

type coderConn struct{ c *websocket.Conn }

func (w *coderConn) Write(ctx context.Context, data []byte) error {
	return w.c.Write(ctx, websocket.MessageText, data)
}

func (w *coderConn) Close(code websocket.StatusCode, reason string) error {
	return w.c.Close(code, reason)
}
