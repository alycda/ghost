package server

import (
	"fmt"
	"os"
	"strconv"
	"strings"
)

// Config is everything ghost-server reads from its environment. Every key is
// GHOST_SERVER_<NAME>; the API key may come from a file instead so a secrets
// manager can mount it.
type Config struct {
	// Listen is the address the HTTP API binds to.
	Listen string
	// APIKey is the one bearer token the server accepts. There is no user
	// login: this server has a single space and a single operator.
	APIKey string
	// PostgresURL is a superuser connection to the cluster's maintenance
	// database (usually "postgres"). Ghost databases are created next to it and
	// bookkeeping lives in its "ghost" schema.
	PostgresURL string
	// PublicHost and PublicPort are what clients are told to connect to. They
	// are not where Postgres listens as far as the server is concerned; behind
	// an ssh tunnel they are the client's end of it.
	PublicHost string
	PublicPort int
	// SpaceID and SpaceName describe the one space every database lives in.
	SpaceID   string
	SpaceName string
	// StorageLimitMiB is reported as the space's quota. It is not enforced.
	StorageLimitMiB int64
	// UserName and UserEmail are reported as the API key's creator.
	UserName  string
	UserEmail string
}

const envPrefix = "GHOST_SERVER_"

// ConfigFromEnv reads the configuration, applying defaults for everything but
// the API key and the Postgres URL, which have no sensible default.
func ConfigFromEnv() (Config, error) {
	cfg := Config{
		Listen:      envOr("LISTEN", "127.0.0.1:8787"),
		PostgresURL: os.Getenv(envPrefix + "POSTGRES_URL"),
		PublicHost:  envOr("PUBLIC_HOST", "127.0.0.1"),
		SpaceID:     envOr("SPACE_ID", "local"),
		SpaceName:   envOr("SPACE_NAME", "ghost-server"),
		UserName:    envOr("USER_NAME", "ghost-server"),
		UserEmail:   os.Getenv(envPrefix + "USER_EMAIL"),
	}

	port, err := strconv.Atoi(envOr("PUBLIC_PORT", "5432"))
	if err != nil || port < 1 || port > 65535 {
		return cfg, fmt.Errorf("%sPUBLIC_PORT must be a TCP port", envPrefix)
	}
	cfg.PublicPort = port

	limit, err := strconv.ParseInt(envOr("STORAGE_LIMIT_MIB", "10240"), 10, 64)
	if err != nil || limit < 0 {
		return cfg, fmt.Errorf("%sSTORAGE_LIMIT_MIB must be a non-negative integer", envPrefix)
	}
	cfg.StorageLimitMiB = limit

	if cfg.PostgresURL == "" {
		return cfg, fmt.Errorf("%sPOSTGRES_URL is required", envPrefix)
	}

	cfg.APIKey = os.Getenv(envPrefix + "API_KEY")
	if path := os.Getenv(envPrefix + "API_KEY_FILE"); cfg.APIKey == "" && path != "" {
		data, err := os.ReadFile(path)
		if err != nil {
			return cfg, fmt.Errorf("reading %sAPI_KEY_FILE: %w", envPrefix, err)
		}
		cfg.APIKey = strings.TrimSpace(string(data))
	}
	if cfg.APIKey == "" {
		return cfg, fmt.Errorf("%sAPI_KEY or %sAPI_KEY_FILE is required", envPrefix, envPrefix)
	}

	return cfg, nil
}

func envOr(name, fallback string) string {
	if v := os.Getenv(envPrefix + name); v != "" {
		return v
	}
	return fallback
}
