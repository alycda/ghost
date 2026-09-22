package api

import (
	"reflect"
	"testing"
)

// Every field of Database must survive the round trip through
// DatabaseWithUsage; a new schema field that one converter forgets fails here.
func TestDatabaseWithUsageRoundTrip(t *testing.T) {
	dbname, password := "db_x", "pw"
	size, storage := DatabaseSizeN1X, int64(7)
	in := Database{
		Dbname: &dbname, Host: "h", ID: "db_x", Name: "n", Password: &password, Port: 5432,
		Size: &size, Status: DatabaseStatusRunning, StorageMib: &storage, Type: DatabaseTypeStandard,
	}
	if got := in.WithUsage(nil).Database(); !reflect.DeepEqual(got, in) {
		t.Fatalf("round trip lost fields:\n got %+v\nwant %+v", got, in)
	}
	for i := 0; i < reflect.TypeFor[Database]().NumField(); i++ {
		if f := reflect.ValueOf(in).Field(i); f.IsZero() {
			t.Errorf("test fixture leaves %s zero, so the round trip cannot prove it survives", reflect.TypeFor[Database]().Field(i).Name)
		}
	}
}
