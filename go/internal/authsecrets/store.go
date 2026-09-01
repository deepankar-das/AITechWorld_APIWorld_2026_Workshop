package authsecrets

import (
	"context"
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

var (
	ErrDatabaseURLMissing = errors.New("DATABASE_URL is not configured")
	ErrEncryptionKeyUnset = errors.New("AA_AUTH_ENC_KEY (or key file) is not configured")
)

type RoleTokens struct {
	AdminToken    string
	ReviewerToken string
	OperatorToken string
}

type encryptedValue struct {
	nonceB64      string
	ciphertextB64 string
}

func LoadTokens(ctx context.Context) (RoleTokens, error) {
	store, err := newStore(ctx)
	if err != nil {
		return RoleTokens{}, err
	}
	defer store.close()

	if err := store.ensureSchema(ctx); err != nil {
		return RoleTokens{}, err
	}

	rows, err := store.pool.Query(ctx, `
		SELECT role, nonce_b64, ciphertext_b64
		FROM auth_secrets
	`)
	if err != nil {
		return RoleTokens{}, fmt.Errorf("load encrypted auth tokens failed: %w", err)
	}
	defer rows.Close()

	var out RoleTokens
	for rows.Next() {
		var role, nonceB64, ciphertextB64 string
		if err := rows.Scan(&role, &nonceB64, &ciphertextB64); err != nil {
			return RoleTokens{}, fmt.Errorf("scan encrypted auth token failed: %w", err)
		}
		plaintext, err := decrypt(store.key, role, nonceB64, ciphertextB64)
		if err != nil {
			return RoleTokens{}, fmt.Errorf("decrypt auth token for role %q failed: %w", role, err)
		}
		switch role {
		case "admin":
			out.AdminToken = plaintext
		case "reviewer":
			out.ReviewerToken = plaintext
		case "operator":
			out.OperatorToken = plaintext
		}
	}
	return out, nil
}

func SaveTokens(ctx context.Context, tokens RoleTokens) error {
	store, err := newStore(ctx)
	if err != nil {
		return err
	}
	defer store.close()

	if err := store.ensureSchema(ctx); err != nil {
		return err
	}

	if err := store.upsertRoleToken(ctx, "admin", tokens.AdminToken); err != nil {
		return err
	}
	if err := store.upsertRoleToken(ctx, "reviewer", tokens.ReviewerToken); err != nil {
		return err
	}
	if err := store.upsertRoleToken(ctx, "operator", tokens.OperatorToken); err != nil {
		return err
	}
	return nil
}

type tokenStore struct {
	pool *pgxpool.Pool
	key  []byte
}

func newStore(ctx context.Context) (*tokenStore, error) {
	connStr := strings.TrimSpace(os.Getenv("DATABASE_URL"))
	if connStr == "" {
		return nil, ErrDatabaseURLMissing
	}

	key, err := loadEncryptionKey()
	if err != nil {
		return nil, err
	}

	pool, err := pgxpool.New(ctx, connStr)
	if err != nil {
		return nil, fmt.Errorf("auth secret DB connect failed: %w", err)
	}
	if err := pool.Ping(ctx); err != nil {
		pool.Close()
		return nil, fmt.Errorf("auth secret DB ping failed: %w", err)
	}
	return &tokenStore{pool: pool, key: key}, nil
}

func (s *tokenStore) close() {
	if s.pool != nil {
		s.pool.Close()
	}
}

func (s *tokenStore) ensureSchema(ctx context.Context) error {
	_, err := s.pool.Exec(ctx, `
		CREATE TABLE IF NOT EXISTS auth_secrets (
			role TEXT PRIMARY KEY CHECK (role IN ('admin', 'reviewer', 'operator')),
			nonce_b64 TEXT NOT NULL,
			ciphertext_b64 TEXT NOT NULL,
			key_version INTEGER NOT NULL DEFAULT 1,
			updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
		)
	`)
	if err == nil {
		return nil
	}

	// Deploy/migration scripts usually create this table with a higher-privilege
	// role and grant only DML to the runtime user. In that case, CREATE TABLE can
	// fail with "permission denied for schema public" even though the table exists.
	// Treat that as acceptable if auth_secrets is already present.
	var exists bool
	existsErr := s.pool.QueryRow(ctx, `SELECT to_regclass('public.auth_secrets') IS NOT NULL`).Scan(&exists)
	if existsErr == nil && exists {
		return nil
	}
	if existsErr != nil {
		return fmt.Errorf("ensure auth_secrets schema failed: %v (table-exists check failed: %w)", err, existsErr)
	}
	return fmt.Errorf("ensure auth_secrets schema failed: %w", err)
}

func (s *tokenStore) upsertRoleToken(ctx context.Context, role string, token string) error {
	if strings.TrimSpace(token) == "" {
		return nil
	}
	ev, err := encrypt(s.key, role, token)
	if err != nil {
		return fmt.Errorf("encrypt auth token for role %q failed: %w", role, err)
	}
	_, err = s.pool.Exec(ctx, `
		INSERT INTO auth_secrets (role, nonce_b64, ciphertext_b64, key_version, updated_at)
		VALUES ($1, $2, $3, 1, $4)
		ON CONFLICT (role) DO UPDATE
		SET nonce_b64 = EXCLUDED.nonce_b64,
		    ciphertext_b64 = EXCLUDED.ciphertext_b64,
		    key_version = EXCLUDED.key_version,
		    updated_at = EXCLUDED.updated_at
	`, role, ev.nonceB64, ev.ciphertextB64, time.Now().UTC())
	if err != nil {
		return fmt.Errorf("upsert auth token for role %q failed: %w", role, err)
	}
	return nil
}

func encrypt(key []byte, role string, plaintext string) (encryptedValue, error) {
	block, err := aes.NewCipher(key)
	if err != nil {
		return encryptedValue{}, err
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return encryptedValue{}, err
	}
	nonce := make([]byte, gcm.NonceSize())
	if _, err := rand.Read(nonce); err != nil {
		return encryptedValue{}, err
	}
	ciphertext := gcm.Seal(nil, nonce, []byte(plaintext), []byte(role))
	return encryptedValue{
		nonceB64:      base64.StdEncoding.EncodeToString(nonce),
		ciphertextB64: base64.StdEncoding.EncodeToString(ciphertext),
	}, nil
}

func decrypt(key []byte, role string, nonceB64 string, ciphertextB64 string) (string, error) {
	nonce, err := base64.StdEncoding.DecodeString(nonceB64)
	if err != nil {
		return "", err
	}
	ciphertext, err := base64.StdEncoding.DecodeString(ciphertextB64)
	if err != nil {
		return "", err
	}
	block, err := aes.NewCipher(key)
	if err != nil {
		return "", err
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return "", err
	}
	plaintext, err := gcm.Open(nil, nonce, ciphertext, []byte(role))
	if err != nil {
		return "", err
	}
	return string(plaintext), nil
}

func loadEncryptionKey() ([]byte, error) {
	if v := strings.TrimSpace(os.Getenv("AA_AUTH_ENC_KEY")); v != "" {
		key, err := parseKey(v)
		if err != nil {
			return nil, fmt.Errorf("AA_AUTH_ENC_KEY invalid: %w", err)
		}
		return key, nil
	}

	keyFile := strings.TrimSpace(os.Getenv("AA_AUTH_ENC_KEY_FILE"))
	if keyFile == "" {
		configDir := strings.TrimSpace(os.Getenv("AA_CONFIG_DIR"))
		if configDir == "" {
			configDir = "/etc/enforcer"
		}
		keyFile = filepath.Join(configDir, ".auth_enc_key")
	}

	data, err := os.ReadFile(keyFile)
	if err != nil {
		return nil, ErrEncryptionKeyUnset
	}
	key, err := parseKey(strings.TrimSpace(string(data)))
	if err != nil {
		return nil, fmt.Errorf("auth key file %s invalid: %w", keyFile, err)
	}
	return key, nil
}

func parseKey(v string) ([]byte, error) {
	if len(v) == 32 {
		return []byte(v), nil
	}
	if raw, err := base64.StdEncoding.DecodeString(v); err == nil && len(raw) == 32 {
		return raw, nil
	}
	if raw, err := hex.DecodeString(v); err == nil && len(raw) == 32 {
		return raw, nil
	}
	return nil, fmt.Errorf("expected 32-byte raw key, base64(32 bytes), or hex(64 chars)")
}
