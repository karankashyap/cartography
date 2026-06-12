package db

import (
	"context"
	"fmt"
	"log/slog"
	"sync"

	"github.com/jackc/pgx/v5/pgxpool"
)

// PubSub wraps Postgres LISTEN/NOTIFY for real-time job progress.
type PubSub struct {
	pool        *pgxpool.Pool
	mu          sync.RWMutex
	subscribers map[string][]chan string
}

func NewPubSub(pool *pgxpool.Pool) *PubSub {
	return &PubSub{
		pool:        pool,
		subscribers: make(map[string][]chan string),
	}
}

// Subscribe registers a channel to receive notifications on the given Postgres channel.
func (p *PubSub) Subscribe(channel string) (<-chan string, func()) {
	ch := make(chan string, 16)

	p.mu.Lock()
	p.subscribers[channel] = append(p.subscribers[channel], ch)
	p.mu.Unlock()

	unsub := func() {
		p.mu.Lock()
		defer p.mu.Unlock()
		subs := p.subscribers[channel]
		for i, s := range subs {
			if s == ch {
				p.subscribers[channel] = append(subs[:i], subs[i+1:]...)
				close(ch)
				break
			}
		}
	}
	return ch, unsub
}

// Notify sends a payload on a Postgres channel.
func (p *PubSub) Notify(ctx context.Context, channel, payload string) error {
	_, err := p.pool.Exec(ctx, fmt.Sprintf("SELECT pg_notify($1, $2)"), channel, payload)
	return err
}

// Listen starts a background goroutine that forwards Postgres notifications
// to registered Go subscribers for the given channel.
func (p *PubSub) Listen(ctx context.Context, channel string) {
	go func() {
		conn, err := p.pool.Acquire(ctx)
		if err != nil {
			slog.Error("pubsub acquire", "err", err)
			return
		}
		defer conn.Release()

		if _, err := conn.Exec(ctx, "LISTEN "+channel); err != nil {
			slog.Error("pubsub listen", "channel", channel, "err", err)
			return
		}

		for {
			notif, err := conn.Conn().WaitForNotification(ctx)
			if err != nil {
				if ctx.Err() != nil {
					return
				}
				slog.Error("pubsub wait", "err", err)
				return
			}

			p.mu.RLock()
			subs := p.subscribers[notif.Channel]
			p.mu.RUnlock()

			for _, ch := range subs {
				select {
				case ch <- notif.Payload:
				default:
				}
			}
		}
	}()
}
