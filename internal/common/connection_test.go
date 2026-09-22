package common

import (
	"testing"

	"github.com/timescale/ghost/internal/api"
)

func TestDatabaseName(t *testing.T) {
	named := "db_abc123"
	empty := ""
	tests := []struct {
		name     string
		database api.Database
		want     string
	}{
		{"absent means tsdb", api.Database{}, "tsdb"},
		{"empty means tsdb", api.Database{Dbname: &empty}, "tsdb"},
		{"named database", api.Database{Dbname: &named}, "db_abc123"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := DatabaseName(tt.database); got != tt.want {
				t.Errorf("DatabaseName() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestBuildConnectionStringUsesDbname(t *testing.T) {
	named := "db_abc123"
	got, err := BuildConnectionString(ConnectionStringArgs{
		Database: api.Database{Host: "127.0.0.1", Port: 5432, Dbname: &named},
		Role:     "tsdbadmin",
		Password: "p@ss",
	})
	if err != nil {
		t.Fatal(err)
	}
	want := "postgresql://tsdbadmin:p%40ss@127.0.0.1:5432/db_abc123?sslmode=require"
	if got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}
