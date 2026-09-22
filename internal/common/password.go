package common

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/jackc/pgpassfile"
	"github.com/timescale/ghost/internal/api"
	"github.com/timescale/ghost/internal/util"
)

// escapePgpassField escapes special characters in a .pgpass field value.
// The .pgpass format uses : as a field delimiter and \ as an escape character,
// so literal \ must be escaped as \\ and literal : must be escaped as \:.
func escapePgpassField(field string) string {
	field = strings.ReplaceAll(field, `\`, `\\`)
	field = strings.ReplaceAll(field, `:`, `\:`)
	return field
}

// ErrPasswordNotFound is returned when no password is available from the API
// and no matching entry exists in .pgpass.
var ErrPasswordNotFound = errors.New("password not found")

// SavePassword saves the database password to the user's ~/.pgpass file.
// Returns an error if saving fails.
func SavePassword(database api.Database, password, role string) error {
	if password == "" {
		return errors.New("password is required")
	}
	if role == "" {
		return errors.New("role is required")
	}

	pgpassPath, err := getPgpassPath()
	if err != nil {
		return err
	}

	host := database.Host
	port := strconv.Itoa(database.Port)
	dbName := DatabaseName(database)

	// Remove existing entry first (if it exists)
	if err := removePgpassEntry(pgpassPath, host, port, dbName, role); err != nil {
		return fmt.Errorf("failed to remove existing .pgpass entry: %w", err)
	}

	// Create new entry: hostname:port:database:username:password
	entry := fmt.Sprintf("%s:%s:%s:%s:%s\n",
		escapePgpassField(host), escapePgpassField(port),
		escapePgpassField(dbName), escapePgpassField(role),
		escapePgpassField(password))

	// Append to .pgpass file with restricted permissions
	file, err := os.OpenFile(pgpassPath, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0600)
	if err != nil {
		return fmt.Errorf("failed to open .pgpass file: %w", err)
	}
	defer file.Close()

	if _, err := file.WriteString(entry); err != nil {
		return fmt.Errorf("failed to write to .pgpass file: %w", err)
	}

	if err := file.Close(); err != nil {
		return fmt.Errorf("failed to close .pgpass file: %w", err)
	}

	return nil
}

// GetPassword retrieves the password for a database. It first checks the
// API-provided password on the Database object, then falls back to the
// user's ~/.pgpass file. Returns ErrPasswordNotFound if neither source
// has a password.
func GetPassword(database api.Database, role string) (string, error) {
	if role == "" {
		return "", errors.New("role is required")
	}

	// Prefer password from API response when available
	if util.Deref(database.Password) != "" {
		return *database.Password, nil
	}

	// Fall back to .pgpass file
	pgpassPath, err := getPgpassPath()
	if err != nil {
		return "", fmt.Errorf("failed to get .pgpass file path: %w", err)
	}

	passfile, err := pgpassfile.ReadPassfile(pgpassPath)
	if err != nil {
		if os.IsNotExist(err) {
			return "", ErrPasswordNotFound
		}
		return "", fmt.Errorf("failed to read .pgpass file: %w", err)
	}

	host := database.Host
	port := strconv.Itoa(database.Port)
	dbName := DatabaseName(database)

	password := passfile.FindPassword(host, port, dbName, role)
	if password == "" {
		return "", ErrPasswordNotFound
	}

	return password, nil
}

// RemovePgpassEntry removes the .pgpass entry for the given database and role.
func RemovePgpassEntry(database api.Database, role string) error {
	if role == "" {
		return errors.New("role is required")
	}

	pgpassPath, err := getPgpassPath()
	if err != nil {
		return err
	}

	host := database.Host
	port := strconv.Itoa(database.Port)
	dbName := DatabaseName(database)
	return removePgpassEntry(pgpassPath, host, port, dbName, role)
}

// getPgpassPath returns the path to the user's ~/.pgpass file.
func getPgpassPath() (string, error) {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("failed to get user home directory: %w", err)
	}
	return filepath.Join(homeDir, ".pgpass"), nil
}

// removePgpassEntry removes an existing entry from the .pgpass file.
func removePgpassEntry(pgpassPath, host, port, dbName, username string) error {
	file, err := os.Open(pgpassPath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil // File doesn't exist, nothing to remove
		}
		return fmt.Errorf("failed to open .pgpass file: %w", err)
	}
	defer file.Close()

	passfile, err := pgpassfile.ParsePassfile(file)
	if err != nil {
		return fmt.Errorf("failed to parse .pgpass file: %w", err)
	}

	if err := file.Close(); err != nil {
		return fmt.Errorf("failed to close .pgpass file: %w", err)
	}

	// Filter out entries that match our target
	var filteredEntries []*pgpassfile.Entry
	for _, entry := range passfile.Entries {
		if !(entry.Hostname == host && entry.Port == port && entry.Database == dbName && entry.Username == username) {
			filteredEntries = append(filteredEntries, entry)
		}
	}

	// Write back all entries except the one we want to remove
	tmpFile, err := os.CreateTemp(filepath.Dir(pgpassPath), ".pgpass.tmp")
	if err != nil {
		return fmt.Errorf("failed to create temporary file: %w", err)
	}
	defer os.Remove(tmpFile.Name())
	defer tmpFile.Close()

	for _, entry := range filteredEntries {
		line := fmt.Sprintf("%s:%s:%s:%s:%s\n",
			escapePgpassField(entry.Hostname), escapePgpassField(entry.Port),
			escapePgpassField(entry.Database), escapePgpassField(entry.Username),
			escapePgpassField(entry.Password))
		if _, err := tmpFile.WriteString(line); err != nil {
			return fmt.Errorf("failed to write to temporary file: %w", err)
		}
	}

	if err := tmpFile.Close(); err != nil {
		return fmt.Errorf("failed to close temporary file: %w", err)
	}

	// Set proper permissions and replace the original file
	if err := os.Chmod(tmpFile.Name(), 0600); err != nil {
		return fmt.Errorf("failed to set permissions on temporary file: %w", err)
	}

	if err := os.Rename(tmpFile.Name(), pgpassPath); err != nil {
		return fmt.Errorf("failed to replace .pgpass file: %w", err)
	}

	return nil
}
