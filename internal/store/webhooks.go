package store

import (
	"context"
	"database/sql"
	"fmt"
	"net/url"
	"strings"
	"time"
)

const maxWebhooks = 20

// Webhook is one operator-managed outbound URL.
type Webhook struct {
	ID        int64
	URL       string
	Secret    string
	Enabled   bool
	Events    string // empty = all; comma-separated event names
	Created   time.Time
	HasSecret bool
}

// ListWebhooks returns all rows (secrets stripped for display; HasSecret set).
func (s *Store) ListWebhooks(ctx context.Context) ([]Webhook, error) {
	rows, err := s.db.QueryContext(ctx,
		`SELECT id, url, secret, enabled, events, created FROM webhooks ORDER BY id`)
	if err != nil {
		return nil, fmt.Errorf("list webhooks: %w", err)
	}
	defer rows.Close()
	var out []Webhook
	for rows.Next() {
		var w Webhook
		var en int
		var created, secret string
		if err := rows.Scan(&w.ID, &w.URL, &secret, &en, &w.Events, &created); err != nil {
			return nil, err
		}
		w.Enabled = en != 0
		w.HasSecret = secret != ""
		w.Created, _ = time.Parse(time.RFC3339Nano, created)
		out = append(out, w)
	}
	return out, rows.Err()
}

// DeliveryWebhooks returns enabled hooks with secrets for POSTing.
func (s *Store) DeliveryWebhooks(ctx context.Context) ([]Webhook, error) {
	rows, err := s.db.QueryContext(ctx,
		`SELECT id, url, secret, enabled, events, created FROM webhooks WHERE enabled = 1 ORDER BY id`)
	if err != nil {
		return nil, fmt.Errorf("delivery webhooks: %w", err)
	}
	defer rows.Close()
	var out []Webhook
	for rows.Next() {
		var w Webhook
		var en int
		var created string
		if err := rows.Scan(&w.ID, &w.URL, &w.Secret, &en, &w.Events, &created); err != nil {
			return nil, err
		}
		w.Enabled = true
		w.HasSecret = w.Secret != ""
		w.Created, _ = time.Parse(time.RFC3339Nano, created)
		out = append(out, w)
	}
	return out, rows.Err()
}

func (s *Store) AddWebhook(ctx context.Context, rawURL, secret, events string) (Webhook, error) {
	u, err := normalizeWebhookURL(rawURL)
	if err != nil {
		return Webhook{}, err
	}
	var n int
	if err := s.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM webhooks`).Scan(&n); err != nil {
		return Webhook{}, err
	}
	if n >= maxWebhooks {
		return Webhook{}, fmt.Errorf("at most %d webhooks", maxWebhooks)
	}
	created := time.Now().UTC().Format(time.RFC3339Nano)
	res, err := s.db.ExecContext(ctx,
		`INSERT INTO webhooks (url, secret, enabled, events, created) VALUES (?, ?, 1, ?, ?)`,
		u, strings.TrimSpace(secret), strings.TrimSpace(events), created,
	)
	if err != nil {
		return Webhook{}, fmt.Errorf("add webhook: %w", err)
	}
	id, _ := res.LastInsertId()
	return Webhook{ID: id, URL: u, Enabled: true, Events: strings.TrimSpace(events), HasSecret: strings.TrimSpace(secret) != ""}, nil
}

func (s *Store) GetWebhook(ctx context.Context, id int64) (Webhook, error) {
	var w Webhook
	var en int
	var created string
	err := s.db.QueryRowContext(ctx,
		`SELECT id, url, secret, enabled, events, created FROM webhooks WHERE id = ?`, id,
	).Scan(&w.ID, &w.URL, &w.Secret, &en, &w.Events, &created)
	if err == sql.ErrNoRows {
		return Webhook{}, ErrNotFound
	}
	if err != nil {
		return Webhook{}, fmt.Errorf("get webhook: %w", err)
	}
	w.Enabled = en != 0
	w.HasSecret = w.Secret != ""
	w.Created, _ = time.Parse(time.RFC3339Nano, created)
	return w, nil
}

func (s *Store) DeleteWebhook(ctx context.Context, id int64) error {
	res, err := s.db.ExecContext(ctx, `DELETE FROM webhooks WHERE id = ?`, id)
	if err != nil {
		return fmt.Errorf("delete webhook: %w", err)
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return ErrNotFound
	}
	return nil
}

func (s *Store) SetWebhookEnabled(ctx context.Context, id int64, on bool) error {
	en := 0
	if on {
		en = 1
	}
	res, err := s.db.ExecContext(ctx, `UPDATE webhooks SET enabled = ? WHERE id = ?`, en, id)
	if err != nil {
		return fmt.Errorf("toggle webhook: %w", err)
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return ErrNotFound
	}
	return nil
}

func normalizeWebhookURL(raw string) (string, error) {
	raw = strings.TrimSpace(raw)
	u, err := url.Parse(raw)
	if err != nil || u.Host == "" || (u.Scheme != "http" && u.Scheme != "https") {
		return "", fmt.Errorf("webhook URL must be http(s)")
	}
	return u.String(), nil
}
