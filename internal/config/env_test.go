package config

import (
	"testing"

	"github.com/spf13/viper"
)

func TestEmptyEnvOverridesOnlyDocsMCPURL(t *testing.T) {
	t.Setenv("GHOST_DOCS_MCP_URL", "")
	t.Setenv("GHOST_READ_ONLY", "")
	v := viper.New()
	applyDefaults(v)
	v.Set("read_only", true) // as if from the config file
	applyEnvOverrides(v)
	if got := v.GetString("docs_mcp_url"); got != "" {
		t.Errorf("docs_mcp_url = %q, want empty", got)
	}
	if !v.GetBool("read_only") {
		t.Error("an empty GHOST_READ_ONLY switched read_only off")
	}
}
