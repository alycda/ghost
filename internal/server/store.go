package server

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"

	"github.com/timescale/ghost/internal/api"
)

// idPrefix starts every database ID; names may not.
const idPrefix = "db_"

// record is one row of ghost.databases joined with what Postgres knows about
// the database behind it.
type record struct {
	ID         string
	Name       string
	Status     string
	CreatedAt  time.Time
	ForkedFrom *string
	Exists     bool // the Postgres database is present
	AllowConn  bool // datallowconn
	SizeBytes  *int64
}

const recordColumns = `d.id, d.name, d.status, d.created_at, d.forked_from,
	pd.oid IS NOT NULL, COALESCE(pd.datallowconn, false),
	CASE WHEN pd.oid IS NOT NULL THEN pg_database_size(pd.oid) END
	FROM ghost.databases d LEFT JOIN pg_database pd ON pd.datname = d.id`

func scanRecord(row pgx.Row) (record, error) {
	var r record
	err := row.Scan(&r.ID, &r.Name, &r.Status, &r.CreatedAt, &r.ForkedFrom, &r.Exists, &r.AllowConn, &r.SizeBytes)
	return r, err
}

// resolve finds a database by ID or name in the one space this server has.
func (s *Server) resolve(ctx context.Context, spaceID, ref string) (record, error) {
	if spaceID != s.cfg.SpaceID {
		return record{}, notFound("space %q not found", spaceID)
	}
	// A ref is an ID or a name. IDs win, and validName keeps names out of
	// the ID namespace, so the two cannot collide in the first place.
	r, err := scanRecord(s.pool.QueryRow(ctx,
		`SELECT `+recordColumns+` WHERE d.id = $1 OR d.name = $1 ORDER BY d.id = $1 DESC LIMIT 1`, ref))
	if errors.Is(err, pgx.ErrNoRows) {
		return record{}, notFound("database %q not found", ref)
	}
	if err != nil {
		return record{}, fmt.Errorf("looking up database %q: %w", ref, err)
	}
	return r, nil
}

func (s *Server) list(ctx context.Context, spaceID string) ([]record, error) {
	if spaceID != s.cfg.SpaceID {
		return nil, notFound("space %q not found", spaceID)
	}
	rows, err := s.pool.Query(ctx, `SELECT `+recordColumns+` ORDER BY d.created_at, d.id`)
	if err != nil {
		return nil, fmt.Errorf("listing databases: %w", err)
	}
	defer rows.Close()
	var out []record
	for rows.Next() {
		r, err := scanRecord(rows)
		if err != nil {
			return nil, fmt.Errorf("reading database row: %w", err)
		}
		out = append(out, r)
	}
	return out, rows.Err()
}

// create reserves the name and ID first, then creates the Postgres database,
// so a name clash is a clean 409 with nothing to undo. With a template, the new
// database is a fork: Postgres copies the template's files, which needs no one
// connected to it, so the template's sessions are ended first.
func (s *Server) create(ctx context.Context, name string, template *record) (record, error) {
	if name == "" {
		var err error
		if name, err = randomName(); err != nil {
			return record{}, err
		}
	}
	if err := validName(name); err != nil {
		return record{}, err
	}
	suffix, err := randomString(12)
	if err != nil {
		return record{}, err
	}
	id := idPrefix + suffix

	var forkedFrom *string
	if template != nil {
		forkedFrom = &template.ID
	}
	if _, err := s.pool.Exec(ctx,
		`INSERT INTO ghost.databases (id, name, status, forked_from) VALUES ($1, $2, $3, $4)`,
		id, name, string(api.DatabaseStatusConfiguring), forkedFrom); err != nil {
		if isUniqueViolation(err) {
			return record{}, conflict("a database named %q already exists", name)
		}
		return record{}, fmt.Errorf("reserving database %q: %w", name, err)
	}

	// undo runs when the client may already be gone (its request context
	// cancelled), so it uses a context that survives that, drops whatever
	// CREATE DATABASE may have left, and says so if it cannot.
	undo := func() {
		cleanup, cancel := context.WithTimeout(context.WithoutCancel(ctx), 30*time.Second)
		defer cancel()
		if _, err := s.pool.Exec(cleanup, "DROP DATABASE IF EXISTS "+pgx.Identifier{id}.Sanitize()+" WITH (FORCE)"); err != nil {
			s.log.Error("could not drop half-created database", "database", id, "error", err.Error())
		}
		if _, err := s.pool.Exec(cleanup, `DELETE FROM ghost.databases WHERE id = $1`, id); err != nil {
			s.log.Error("could not forget half-created database", "database", id, "error", err.Error())
		}
	}

	statement := "CREATE DATABASE " + pgx.Identifier{id}.Sanitize() + " OWNER " + adminRole
	// What Postgres copies: the template's label for the client's benefit, or
	// template1, which a plain CREATE DATABASE copies implicitly and which any
	// other session on it makes "in use" too.
	source := "template1"
	if template != nil {
		source = template.Name
		// The copy needs the source free of sessions for its whole duration.
		// Ending them is not enough on its own: a reconnecting pool is back
		// before CREATE DATABASE looks. So the source refuses connections
		// until the copy is done, then goes back to how it was.
		if template.AllowConn {
			if err := s.setAllowConnectionsRaw(ctx, template.ID, false); err != nil {
				undo()
				return record{}, err
			}
			defer func() {
				restore, cancel := context.WithTimeout(context.WithoutCancel(ctx), 30*time.Second)
				defer cancel()
				if err := s.setAllowConnectionsRaw(restore, template.ID, true); err != nil {
					s.log.Error("source database left refusing connections after fork", "database", template.ID, "error", err.Error())
				}
			}()
		}
		if err := s.terminateSessions(ctx, template.ID); err != nil {
			undo()
			return record{}, err
		}
		statement += " TEMPLATE " + pgx.Identifier{template.ID}.Sanitize()
	}
	if _, err := s.pool.Exec(ctx, statement); err != nil {
		undo()
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == "55006" { // object_in_use
			return record{}, conflict("database %q is in use; try again", source)
		}
		return record{}, fmt.Errorf("creating database %s: %w", id, err)
	}

	// A fork inherits the template's extensions; a fresh database gets
	// TimescaleDB when the cluster has it. Failing that is not fatal: the
	// database is still a working Postgres.
	if template == nil && s.timescale {
		if err := s.createTimescale(ctx, id); err != nil {
			s.log.Warn("could not create timescaledb extension", "database", id, "error", err.Error())
		}
	}

	if _, err := s.pool.Exec(ctx, `UPDATE ghost.databases SET status = $2 WHERE id = $1`,
		id, string(api.DatabaseStatusRunning)); err != nil {
		undo()
		return record{}, fmt.Errorf("marking %s running: %w", id, err)
	}
	return s.resolve(ctx, s.cfg.SpaceID, id)
}

// createTimescale connects to the new database as the server's own user and
// installs the extension there; CREATE EXTENSION is per database.
func (s *Server) createTimescale(ctx context.Context, id string) error {
	cfg, err := pgx.ParseConfig(s.cfg.PostgresURL)
	if err != nil {
		return err
	}
	cfg.Database = id
	conn, err := pgx.ConnectConfig(ctx, cfg)
	if err != nil {
		return err
	}
	defer conn.Close(ctx)
	_, err = conn.Exec(ctx, `CREATE EXTENSION IF NOT EXISTS timescaledb`)
	return err
}

func (s *Server) drop(ctx context.Context, r record) error {
	if r.Exists {
		if _, err := s.pool.Exec(ctx, "DROP DATABASE "+pgx.Identifier{r.ID}.Sanitize()+" WITH (FORCE)"); err != nil {
			return fmt.Errorf("dropping database %s: %w", r.ID, err)
		}
	}
	if _, err := s.pool.Exec(ctx, `DELETE FROM ghost.databases WHERE id = $1`, r.ID); err != nil {
		return fmt.Errorf("forgetting database %s: %w", r.ID, err)
	}
	return nil
}

// setAllowConnections pauses (false) or resumes (true) a database. Pausing also
// ends the sessions that are already there.
func (s *Server) setAllowConnections(ctx context.Context, r record, allow bool) error {
	if !r.Exists {
		return notFound("database %q has no Postgres database behind it", r.Name)
	}
	if err := s.setAllowConnectionsRaw(ctx, r.ID, allow); err != nil {
		return err
	}
	status := api.DatabaseStatusRunning
	if !allow {
		status = api.DatabaseStatusPaused
		if err := s.terminateSessions(ctx, r.ID); err != nil {
			return err
		}
	}
	if _, err := s.pool.Exec(ctx, `UPDATE ghost.databases SET status = $2 WHERE id = $1`, r.ID, string(status)); err != nil {
		return fmt.Errorf("updating status of %s: %w", r.ID, err)
	}
	return nil
}

// validName rejects labels that could be mistaken for IDs. Everything else
// is allowed: the label is never a Postgres identifier.
func validName(name string) error {
	if name == "" {
		return badRequest("name must not be empty")
	}
	if strings.HasPrefix(name, idPrefix) {
		return badRequest("names may not start with %q; that is the ID namespace", idPrefix)
	}
	return nil
}

func (s *Server) rename(ctx context.Context, r record, name string) error {
	if err := validName(name); err != nil {
		return err
	}
	if _, err := s.pool.Exec(ctx, `UPDATE ghost.databases SET name = $2 WHERE id = $1`, r.ID, name); err != nil {
		if isUniqueViolation(err) {
			return conflict("a database named %q already exists", name)
		}
		return fmt.Errorf("renaming %s: %w", r.ID, err)
	}
	return nil
}

func (s *Server) setAllowConnectionsRaw(ctx context.Context, id string, allow bool) error {
	if _, err := s.pool.Exec(ctx, fmt.Sprintf("ALTER DATABASE %s WITH ALLOW_CONNECTIONS %t",
		pgx.Identifier{id}.Sanitize(), allow)); err != nil {
		return fmt.Errorf("setting allow_connections on %s: %w", id, err)
	}
	return nil
}

func (s *Server) terminateSessions(ctx context.Context, id string) error {
	if _, err := s.pool.Exec(ctx,
		`SELECT pg_terminate_backend(pid) FROM pg_stat_activity WHERE datname = $1 AND pid <> pg_backend_pid()`, id); err != nil {
		return fmt.Errorf("ending sessions on %s: %w", id, err)
	}
	return nil
}

func (s *Server) totalStorageBytes(ctx context.Context) (int64, error) {
	var total int64
	err := s.pool.QueryRow(ctx,
		`SELECT COALESCE(SUM(pg_database_size(pd.oid)), 0)::bigint
		 FROM ghost.databases d JOIN pg_database pd ON pd.datname = d.id`).Scan(&total)
	if err != nil {
		return 0, fmt.Errorf("summing storage: %w", err)
	}
	return total, nil
}

func isUniqueViolation(err error) bool {
	var pgErr *pgconn.PgError
	return errors.As(err, &pgErr) && pgErr.Code == "23505"
}
