package mcp

import (
	"context"

	"github.com/google/jsonschema-go/jsonschema"
	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/timescale/ghost/internal/api"
	"github.com/timescale/ghost/internal/util"
)

// ForkDedicatedInput represents input for ghost_fork_dedicated
type ForkDedicatedInput struct {
	Ref  string `json:"name_or_id"`
	Name string `json:"name,omitempty"`
	Size string `json:"size,omitempty"`
	Wait bool   `json:"wait,omitempty"`
}

func (ForkDedicatedInput) Schema() *jsonschema.Schema {
	schema := util.Must(jsonschema.For[ForkDedicatedInput](nil))
	databaseRefInputProperties(schema)
	forkNameInputProperties(schema)
	sizeInputProperties(schema)
	waitInputProperties(schema)
	return schema
}

// ForkDedicatedOutput represents output for ghost_fork_dedicated
type ForkDedicatedOutput struct {
	ID               string   `json:"id"`
	Name             string   `json:"name"`
	Size             string   `json:"size"`
	ConnectionString string   `json:"connection_string"`
	Warnings         []string `json:"warnings,omitempty"`
}

func (ForkDedicatedOutput) Schema() *jsonschema.Schema {
	schema := util.Must(jsonschema.For[ForkDedicatedOutput](nil))
	databaseIDOutputProperties(schema)
	databaseNameOutputProperties(schema)
	sizeOutputProperties(schema)
	connectionStringOutputProperties(schema)
	warningsOutputProperties(schema)
	return schema
}

func newForkDedicatedTool() *mcp.Tool {
	return &mcp.Tool{
		Name:  "ghost_fork_dedicated",
		Title: "Fork Database as Dedicated",
		Description: `Fork an existing database as a new dedicated instance. The fork inherits
the source database's data but runs as an always-on, billed instance.
A payment method must be on file.

Use ghost_pricing to see compute and storage pricing.

Note: forked databases may take a few minutes to start up. Use ghost_list to check the current status.

Against a self-hosted ghost-server, forking ends every open session on the source database for the duration of the copy.`,
		InputSchema:  ForkDedicatedInput{}.Schema(),
		OutputSchema: ForkDedicatedOutput{}.Schema(),
		Annotations: &mcp.ToolAnnotations{
			ReadOnlyHint:    false,
			DestructiveHint: new(true),
			IdempotentHint:  false,
			OpenWorldHint:   new(false),
			Title:           "Fork Database as Dedicated",
		},
	}
}

func (s *Server) handleForkDedicated(ctx context.Context, req *mcp.CallToolRequest, input ForkDedicatedInput) (*mcp.CallToolResult, ForkDedicatedOutput, error) {
	result, err := s.forkDatabase(ctx, forkDatabaseArgs{
		sourceDatabaseRef: input.Ref,
		req: api.ForkDatabaseRequest{
			Name: util.PtrIfNonZero(input.Name),
			Type: new(api.DatabaseTypeDedicated),
			Size: new(api.DatabaseSize(input.Size)),
		},
		wait: input.Wait,
	})
	if err != nil {
		return nil, ForkDedicatedOutput{}, err
	}

	return nil, ForkDedicatedOutput{
		ID:               result.id,
		Name:             result.name,
		Size:             result.size,
		ConnectionString: result.connectionString,
		Warnings:         result.warnings,
	}, nil
}
