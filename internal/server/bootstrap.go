package server

import (
	"context"
	"crypto/rand"
	"fmt"
	"math/big"
	"strings"
	"time"
)

// adminRole is the one Postgres role the CLI connects as. Its name is a
// constant in the CLI, so it is one here too.
const adminRole = "tsdbadmin"

const (
	settingAdminPassword = "tsdbadmin_password"
	settingCreatedAt     = "created_at"
)

// bootstrap makes the cluster ready: the bookkeeping schema, the admin role
// with a password the server knows, and a note of when all this began.
func (s *Server) bootstrap(ctx context.Context) error {
	const schema = `
CREATE SCHEMA IF NOT EXISTS ghost;
CREATE TABLE IF NOT EXISTS ghost.databases (
	id          text PRIMARY KEY,
	name        text NOT NULL UNIQUE,
	status      text NOT NULL,
	created_at  timestamptz NOT NULL DEFAULT now(),
	forked_from text
);
CREATE TABLE IF NOT EXISTS ghost.settings (
	key        text PRIMARY KEY,
	value      text NOT NULL,
	updated_at timestamptz NOT NULL DEFAULT now()
);`
	if _, err := s.pool.Exec(ctx, schema); err != nil {
		return fmt.Errorf("creating ghost schema: %w", err)
	}

	// created_at is written once and then left alone.
	if _, err := s.pool.Exec(ctx,
		`INSERT INTO ghost.settings (key, value) VALUES ($1, $2) ON CONFLICT (key) DO NOTHING`,
		settingCreatedAt, time.Now().UTC().Format(time.RFC3339)); err != nil {
		return fmt.Errorf("recording created_at: %w", err)
	}
	var created string
	if err := s.pool.QueryRow(ctx, `SELECT value FROM ghost.settings WHERE key = $1`, settingCreatedAt).Scan(&created); err != nil {
		return fmt.Errorf("reading created_at: %w", err)
	}
	if t, err := time.Parse(time.RFC3339, created); err == nil {
		s.createdAt = t
	} else {
		s.createdAt = time.Now().UTC()
	}

	// The role must exist and the server must know its password, because the
	// API hands that password to clients. Either missing means a new password.
	var roleExists, passwordKnown bool
	if err := s.pool.QueryRow(ctx, `SELECT EXISTS (SELECT 1 FROM pg_roles WHERE rolname = $1)`, adminRole).Scan(&roleExists); err != nil {
		return fmt.Errorf("checking for role %s: %w", adminRole, err)
	}
	if err := s.pool.QueryRow(ctx, `SELECT EXISTS (SELECT 1 FROM ghost.settings WHERE key = $1)`, settingAdminPassword).Scan(&passwordKnown); err != nil {
		return fmt.Errorf("checking for stored password: %w", err)
	}
	switch {
	case !roleExists:
		password, err := randomString(32)
		if err != nil {
			return err
		}
		verifier, err := scramVerifier(password)
		if err != nil {
			return err
		}
		if err := s.execFormatted(ctx, "CREATE ROLE "+adminRole+" LOGIN PASSWORD %L", verifier); err != nil {
			return fmt.Errorf("creating role %s: %w", adminRole, err)
		}
		if err := s.storeSetting(ctx, settingAdminPassword, password); err != nil {
			return err
		}
		s.log.Info("created admin role", "role", adminRole)
	case !passwordKnown:
		// The role exists but this server did not create it, or the URL now
		// points at a different maintenance database than the one holding the
		// settings. Resetting the password would cut off whoever uses the
		// role today, so that needs an explicit go-ahead.
		if !s.cfg.AdoptExistingRole {
			return fmt.Errorf("role %s exists but its password is not in ghost.settings; set %sADOPT_EXISTING_ROLE=true to reset it, or point %sPOSTGRES_URL at the database that has the settings", adminRole, envPrefix, envPrefix)
		}
		password, err := randomString(32)
		if err != nil {
			return err
		}
		if err := s.setAdminPassword(ctx, password); err != nil {
			return err
		}
		s.log.Warn("adopted an existing role by resetting its password", "role", adminRole)
	}

	if err := s.pool.QueryRow(ctx,
		`SELECT EXISTS (SELECT 1 FROM pg_available_extensions WHERE name = 'timescaledb')`).Scan(&s.timescale); err != nil {
		return fmt.Errorf("checking for timescaledb: %w", err)
	}
	if !s.timescale {
		s.log.Info("timescaledb is not available in this cluster; databases are plain Postgres")
	}
	return nil
}

// execFormatted runs a statement that cannot take bind parameters (DDL), with
// the values quoted by Postgres itself via format(). Use %L for literals and
// %I for identifiers.
func (s *Server) execFormatted(ctx context.Context, template string, args ...any) error {
	params := make([]string, len(args))
	for i := range args {
		params[i] = fmt.Sprintf("$%d::text", i+1)
	}
	var query strings.Builder
	query.WriteString("SELECT format($" + fmt.Sprint(len(args)+1) + "::text")
	for _, p := range params {
		query.WriteString(", " + p)
	}
	query.WriteString(")")
	var statement string
	if err := s.pool.QueryRow(ctx, query.String(), append(args, template)...).Scan(&statement); err != nil {
		return fmt.Errorf("formatting statement: %w", err)
	}
	_, err := s.pool.Exec(ctx, statement)
	return err
}

func (s *Server) storeSetting(ctx context.Context, key, value string) error {
	_, err := s.pool.Exec(ctx,
		`INSERT INTO ghost.settings (key, value) VALUES ($1, $2)
		 ON CONFLICT (key) DO UPDATE SET value = EXCLUDED.value, updated_at = now()`, key, value)
	if err != nil {
		return fmt.Errorf("storing setting %s: %w", key, err)
	}
	return nil
}

// setAdminPassword changes the shared role's password and remembers it. Every
// Ghost database in the cluster is affected, since they share the role.
//
// Postgres gets the SCRAM verifier, not the password: ALTER ROLE cannot take
// bind parameters, so it travels as a plain statement, and a plain statement
// is what log_statement and pg_stat_activity record.
func (s *Server) setAdminPassword(ctx context.Context, password string) error {
	verifier, err := scramVerifier(password)
	if err != nil {
		return err
	}
	if err := s.execFormatted(ctx, "ALTER ROLE "+adminRole+" WITH PASSWORD %L", verifier); err != nil {
		return fmt.Errorf("setting password for %s: %w", adminRole, err)
	}
	return s.storeSetting(ctx, settingAdminPassword, password)
}

func (s *Server) adminPassword(ctx context.Context) (string, error) {
	var password string
	if err := s.pool.QueryRow(ctx, `SELECT value FROM ghost.settings WHERE key = $1`, settingAdminPassword).Scan(&password); err != nil {
		return "", fmt.Errorf("reading stored password: %w", err)
	}
	return password, nil
}

const alphanumeric = "abcdefghijklmnopqrstuvwxyz0123456789"

// randomString draws n characters from a lowercase alphanumeric alphabet, so
// the result is safe as a Postgres identifier and in a URL.
func randomString(n int) (string, error) {
	out := make([]byte, n)
	max := big.NewInt(int64(len(alphanumeric)))
	for i := range out {
		k, err := rand.Int(rand.Reader, max)
		if err != nil {
			return "", fmt.Errorf("generating random string: %w", err)
		}
		out[i] = alphanumeric[k.Int64()]
	}
	return string(out), nil
}
