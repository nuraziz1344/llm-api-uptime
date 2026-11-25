package repo

import (
	"context"
	"database/sql"
	"errors"
	"strconv"
	"time"

	"llm-api-uptime/internal/domain"
)

type SettingsRepo struct {
	writer *sql.DB
	reader *sql.DB
}

func NewSettingsRepo(writer, reader *sql.DB) *SettingsRepo {
	return &SettingsRepo{writer: writer, reader: reader}
}

func (r *SettingsRepo) Get(ctx context.Context, key string) (string, bool, error) {
	var val string
	err := r.reader.QueryRowContext(ctx, `SELECT value FROM settings WHERE key = ?`, key).Scan(&val)
	if errors.Is(err, sql.ErrNoRows) {
		return "", false, nil
	}
	if err != nil {
		return "", false, err
	}
	return val, true, nil
}

func (r *SettingsRepo) Set(ctx context.Context, key, value string) error {
	now := time.Now().UTC().Format(time.RFC3339Nano)
	_, err := r.writer.ExecContext(ctx, `
		INSERT INTO settings (key, value, updated_at) VALUES (?, ?, ?)
		ON CONFLICT(key) DO UPDATE SET value = excluded.value, updated_at = excluded.updated_at
	`, key, value, now)
	return err
}

func (r *SettingsRepo) IsSetupCompleted(ctx context.Context) (bool, string, error) {
	val, ok, err := r.Get(ctx, "setup_completed")
	if err != nil || !ok || val != "true" {
		return false, "", err
	}
	user, _, err := r.Get(ctx, "admin_username")
	if err != nil {
		return false, "", err
	}
	return true, user, nil
}

func (r *SettingsRepo) SaveAdminSetup(ctx context.Context, username, passwordHash, jwtSecret string) error {
	tx, err := r.writer.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()

	now := time.Now().UTC().Format(time.RFC3339Nano)
	pairs := map[string]string{
		"admin_username":          username,
		"admin_password_hash":     passwordHash,
		"jwt_secret":              jwtSecret,
		"setup_completed":         "true",
		"retention_days":          "30",
		"public_page_title":       "LLM API Status",
		"public_page_description": "Live uptime, latency, and generation speed monitoring for LLM endpoints.",
	}

	for k, v := range pairs {
		if _, err := tx.ExecContext(ctx, `
			INSERT INTO settings (key, value, updated_at) VALUES (?, ?, ?)
			ON CONFLICT(key) DO UPDATE SET value = excluded.value, updated_at = excluded.updated_at
		`, k, v, now); err != nil {
			return err
		}
	}
	return tx.Commit()
}

func (r *SettingsRepo) GetAppSettings(ctx context.Context) (domain.AppSettings, error) {
	cfg := domain.AppSettings{
		RetentionDays:         30,
		PublicPageTitle:       "LLM API Status",
		PublicPageDescription: "Live uptime, latency, and generation speed monitoring for LLM endpoints.",
	}
	if v, ok, err := r.Get(ctx, "retention_days"); err != nil {
		return cfg, err
	} else if ok {
		if parsed, err := strconv.Atoi(v); err == nil && parsed >= 1 && parsed <= 365 {
			cfg.RetentionDays = parsed
		}
	}
	if v, ok, err := r.Get(ctx, "public_page_title"); err != nil {
		return cfg, err
	} else if ok && v != "" {
		cfg.PublicPageTitle = v
	}
	if v, ok, err := r.Get(ctx, "public_page_description"); err != nil {
		return cfg, err
	} else if ok {
		cfg.PublicPageDescription = v
	}
	return cfg, nil
}

func (r *SettingsRepo) UpdateAppSettings(ctx context.Context, s domain.AppSettings) error {
	if s.RetentionDays < 1 {
		s.RetentionDays = 1
	} else if s.RetentionDays > 365 {
		s.RetentionDays = 365
	}
	if err := r.Set(ctx, "retention_days", strconv.Itoa(s.RetentionDays)); err != nil {
		return err
	}
	if err := r.Set(ctx, "public_page_title", s.PublicPageTitle); err != nil {
		return err
	}
	return r.Set(ctx, "public_page_description", s.PublicPageDescription)
}
