package server

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"

	"github.com/timescale/ghost/internal/api"
)

// testPostgresURL names a superuser connection to a throwaway cluster. The test
// creates and drops databases there; it is skipped when the variable is unset.
const testPostgresURL = "GHOST_SERVER_TEST_POSTGRES_URL"

const testKey = "gt_test_key"

func newTestServer(t *testing.T) (*Server, api.ClientWithResponsesInterface, string) {
	t.Helper()
	pgURL := os.Getenv(testPostgresURL)
	if pgURL == "" {
		t.Skipf("%s not set", testPostgresURL)
	}
	pgCfg, err := pgx.ParseConfig(pgURL)
	if err != nil {
		t.Fatal(err)
	}
	cfg := Config{
		APIKey:          testKey,
		PostgresURL:     pgURL,
		PublicHost:      pgCfg.Host,
		PublicPort:      int(pgCfg.Port),
		SpaceID:         "local",
		SpaceName:       "test space",
		StorageLimitMiB: 1024,
		UserName:        "tester",
	}
	s, err := New(context.Background(), cfg, slog.New(slog.NewTextHandler(io.Discard, nil)))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(s.Close)
	ts := httptest.NewServer(s.Handler())
	t.Cleanup(ts.Close)
	client, err := api.NewGhostClient(ts.URL+"/v0", api.APIKeyAuth(testKey))
	if err != nil {
		t.Fatal(err)
	}
	return s, client, ts.URL
}

// connectAs opens a TLS connection the way the CLI would, as tsdbadmin.
func connectAs(t *testing.T, db api.Database) (*pgx.Conn, error) {
	t.Helper()
	dsn := fmt.Sprintf("postgresql://%s:%s@%s:%d/%s?sslmode=require",
		adminRole, url.QueryEscape(*db.Password), db.Host, db.Port, *db.Dbname)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	return pgx.Connect(ctx, dsn)
}

func TestLifecycle(t *testing.T) {
	s, client, base := newTestServer(t)
	ctx := context.Background()
	unique := fmt.Sprintf("%d", time.Now().UnixNano())

	// Databases this test creates are dropped even if an assertion fails.
	t.Cleanup(func() {
		records, _ := s.list(ctx, "local")
		for _, r := range records {
			if len(r.Name) > len(unique) && r.Name[len(r.Name)-len(unique):] == unique {
				_ = s.drop(ctx, r)
			}
		}
	})

	t.Run("health needs no key", func(t *testing.T) {
		resp, err := http.Get(base + "/v0/health")
		if err != nil {
			t.Fatal(err)
		}
		if resp.StatusCode != 200 {
			t.Fatalf("health: %d", resp.StatusCode)
		}
	})

	t.Run("everything else does", func(t *testing.T) {
		resp, err := http.Get(base + "/v0/spaces")
		if err != nil {
			t.Fatal(err)
		}
		if resp.StatusCode != 401 {
			t.Fatalf("unauthenticated list spaces: %d", resp.StatusCode)
		}
	})

	t.Run("auth info names the space", func(t *testing.T) {
		resp, err := client.AuthInfoWithResponse(ctx)
		if err != nil {
			t.Fatal(err)
		}
		if resp.JSON200 == nil || resp.JSON200.Type != api.AuthInfoTypeAPIKey || resp.JSON200.APIKey.SpaceID != "local" {
			t.Fatalf("auth info: %s", resp.Body)
		}
	})

	alphaName := "alpha-" + unique
	var alpha api.Database
	t.Run("create", func(t *testing.T) {
		resp, err := client.CreateDatabaseWithResponse(ctx, "local", api.CreateDatabaseRequest{Name: &alphaName})
		if err != nil {
			t.Fatal(err)
		}
		if resp.StatusCode() != 202 {
			t.Fatalf("create: %d %s", resp.StatusCode(), resp.Body)
		}
		alpha = *resp.JSON202
		if alpha.Status != api.DatabaseStatusRunning || alpha.Dbname == nil || *alpha.Dbname != alpha.ID || alpha.Password == nil {
			t.Fatalf("create returned %+v", alpha)
		}
		conn, err := connectAs(t, alpha)
		if err != nil {
			t.Fatalf("connecting as %s: %v", adminRole, err)
		}
		defer conn.Close(ctx)
		if _, err := conn.Exec(ctx, `CREATE TABLE t (x int); INSERT INTO t VALUES (1), (2)`); err != nil {
			t.Fatal(err)
		}
	})

	t.Run("duplicate name is a conflict", func(t *testing.T) {
		resp, err := client.CreateDatabaseWithResponse(ctx, "local", api.CreateDatabaseRequest{Name: &alphaName})
		if err != nil {
			t.Fatal(err)
		}
		if resp.StatusCode() != 409 {
			t.Fatalf("duplicate create: %d %s", resp.StatusCode(), resp.Body)
		}
	})

	betaName := "beta-" + unique
	var beta api.Database
	t.Run("fork copies the data", func(t *testing.T) {
		resp, err := client.ForkDatabaseWithResponse(ctx, "local", alphaName, api.ForkDatabaseRequest{Name: &betaName})
		if err != nil {
			t.Fatal(err)
		}
		if resp.StatusCode() != 202 {
			t.Fatalf("fork: %d %s", resp.StatusCode(), resp.Body)
		}
		beta = *resp.JSON202
		conn, err := connectAs(t, beta)
		if err != nil {
			t.Fatal(err)
		}
		defer conn.Close(ctx)
		var n int
		if err := conn.QueryRow(ctx, `SELECT count(*) FROM t`).Scan(&n); err != nil || n != 2 {
			t.Fatalf("fork has %d rows, err %v", n, err)
		}
	})

	t.Run("list shows both with storage", func(t *testing.T) {
		resp, err := client.ListDatabasesWithResponse(ctx, "local")
		if err != nil {
			t.Fatal(err)
		}
		found := 0
		for _, db := range *resp.JSON200 {
			if db.ID == alpha.ID || db.ID == beta.ID {
				found++
				if db.StorageMib == nil {
					t.Errorf("%s has no storage figure", db.Name)
				}
			}
		}
		if found != 2 {
			t.Fatalf("found %d of 2", found)
		}
	})

	t.Run("pause refuses connections, resume allows them", func(t *testing.T) {
		resp, err := client.PauseDatabaseWithResponse(ctx, "local", alpha.ID)
		if err != nil {
			t.Fatal(err)
		}
		if resp.StatusCode() != 202 || resp.JSON202.Status != api.DatabaseStatusPaused {
			t.Fatalf("pause: %d %s", resp.StatusCode(), resp.Body)
		}
		if conn, err := connectAs(t, alpha); err == nil {
			conn.Close(ctx)
			t.Fatal("connected to a paused database")
		}
		get, err := client.GetDatabaseWithResponse(ctx, "local", alphaName)
		if err != nil || get.JSON200.Status != api.DatabaseStatusPaused {
			t.Fatalf("get after pause: %v %s", err, get.Body)
		}
		res, err := client.ResumeDatabaseWithResponse(ctx, "local", alpha.ID)
		if err != nil || res.StatusCode() != 202 || res.JSON202.Status != api.DatabaseStatusRunning {
			t.Fatalf("resume: %v %s", err, res.Body)
		}
		conn, err := connectAs(t, alpha)
		if err != nil {
			t.Fatalf("connect after resume: %v", err)
		}
		conn.Close(ctx)
	})

	gammaName := "gamma-" + unique
	t.Run("rename is by label", func(t *testing.T) {
		resp, err := client.RenameDatabaseWithResponse(ctx, "local", betaName, api.RenameDatabaseRequest{Name: gammaName})
		if err != nil || resp.StatusCode() != 204 {
			t.Fatalf("rename: %v %d %s", err, resp.StatusCode(), resp.Body)
		}
		get, err := client.GetDatabaseWithResponse(ctx, "local", gammaName)
		if err != nil || get.JSON200 == nil || get.JSON200.ID != beta.ID || *get.JSON200.Dbname != beta.ID {
			t.Fatalf("get after rename: %v %s", err, get.Body)
		}
		clash, err := client.RenameDatabaseWithResponse(ctx, "local", gammaName, api.RenameDatabaseRequest{Name: alphaName})
		if err != nil || clash.StatusCode() != 409 {
			t.Fatalf("rename onto existing name: %v %d", err, clash.StatusCode())
		}
		idLike, err := client.RenameDatabaseWithResponse(ctx, "local", gammaName, api.RenameDatabaseRequest{Name: alpha.ID})
		if err != nil || idLike.StatusCode() != 400 {
			t.Fatalf("rename into the ID namespace: %v %d", err, idLike.StatusCode())
		}
	})

	t.Run("password change reaches the role", func(t *testing.T) {
		resp, err := client.UpdatePasswordWithResponse(ctx, "local", alpha.ID, api.UpdatePasswordRequest{Password: "new-" + unique})
		if err != nil || resp.StatusCode() != 204 {
			t.Fatalf("password: %v %d %s", err, resp.StatusCode(), resp.Body)
		}
		get, err := client.GetDatabaseWithResponse(ctx, "local", alpha.ID)
		if err != nil || *get.JSON200.Password != "new-"+unique {
			t.Fatalf("password not reported back: %v %s", err, get.Body)
		}
		conn, err := connectAs(t, *get.JSON200)
		if err != nil {
			t.Fatalf("connect with new password: %v", err)
		}
		conn.Close(ctx)
	})

	t.Run("usage sums storage", func(t *testing.T) {
		resp, err := client.SpaceUsageWithResponse(ctx, "local")
		if err != nil || resp.JSON200 == nil || resp.JSON200.StorageMib < 10 || resp.JSON200.StorageLimitMib != 1024 {
			t.Fatalf("usage: %v %s", err, resp.Body)
		}
	})

	t.Run("unknown ref is 404, unsupported op is 501", func(t *testing.T) {
		resp, err := client.GetDatabaseWithResponse(ctx, "local", "nope-"+unique)
		if err != nil || resp.StatusCode() != 404 {
			t.Fatalf("unknown ref: %v %d", err, resp.StatusCode())
		}
		inv, err := client.ListInvoicesWithResponse(ctx, "local")
		if err != nil || inv.StatusCode() != 501 || inv.JSONDefault == nil {
			t.Fatalf("invoices: %v %d %s", err, inv.StatusCode(), inv.Body)
		}
	})

	t.Run("delete drops the database", func(t *testing.T) {
		for _, ref := range []string{alpha.ID, gammaName} {
			resp, err := client.DeleteDatabaseWithResponse(ctx, "local", ref)
			if err != nil || resp.StatusCode() != 202 || resp.JSON202.Status != api.DatabaseStatusDeleted {
				t.Fatalf("delete %s: %v %d %s", ref, err, resp.StatusCode(), resp.Body)
			}
		}
		if conn, err := connectAs(t, alpha); err == nil {
			conn.Close(ctx)
			t.Fatal("connected to a deleted database")
		}
		get, err := client.GetDatabaseWithResponse(ctx, "local", alpha.ID)
		if err != nil || get.StatusCode() != 404 {
			t.Fatalf("get after delete: %v %d", err, get.StatusCode())
		}
	})
}
