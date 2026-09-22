package mcp

import (
	"context"
	"errors"
	"fmt"
	"net/http"

	"github.com/google/jsonschema-go/jsonschema"
	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/timescale/ghost/internal/api"
	"github.com/timescale/ghost/internal/common"
	"github.com/timescale/ghost/internal/util"
)

// ForkInput represents input for ghost_fork
type ForkInput struct {
	Ref  string `json:"name_or_id"`
	Name string `json:"name,omitempty"`
	Wait bool   `json:"wait,omitempty"`
}

func (ForkInput) Schema() *jsonschema.Schema {
	schema := util.Must(jsonschema.For[ForkInput](nil))
	databaseRefInputProperties(schema)
	forkNameInputProperties(schema)
	waitInputProperties(schema)
	return schema
}

// ForkOutput represents output for ghost_fork
type ForkOutput struct {
	ID               string   `json:"id"`
	Name             string   `json:"name"`
	ConnectionString string   `json:"connection_string"`
	Warnings         []string `json:"warnings,omitempty"`
}

func (ForkOutput) Schema() *jsonschema.Schema {
	schema := util.Must(jsonschema.For[ForkOutput](nil))
	databaseIDOutputProperties(schema)
	databaseNameOutputProperties(schema)
	connectionStringOutputProperties(schema)
	warningsOutputProperties(schema)
	return schema
}

func newForkTool() *mcp.Tool {
	return &mcp.Tool{
		Name:  "ghost_fork",
		Title: "Fork Database",
		Description: `Fork an existing database to create a new independent copy.

Note: forked databases may take a few minutes to start up. Use ghost_list to check the current status.

Against a self-hosted ghost-server, forking ends every open session on the source database for the duration of the copy.`,
		InputSchema:  ForkInput{}.Schema(),
		OutputSchema: ForkOutput{}.Schema(),
		Annotations: &mcp.ToolAnnotations{
			ReadOnlyHint:    false,
			DestructiveHint: new(true),
			IdempotentHint:  false,
			OpenWorldHint:   new(false),
			Title:           "Fork Database",
		},
	}
}

func (s *Server) handleFork(ctx context.Context, req *mcp.CallToolRequest, input ForkInput) (*mcp.CallToolResult, ForkOutput, error) {
	result, err := s.forkDatabase(ctx, forkDatabaseArgs{
		sourceDatabaseRef: input.Ref,
		req: api.ForkDatabaseRequest{
			Name: util.PtrIfNonZero(input.Name),
		},
		wait: input.Wait,
	})
	if err != nil {
		return nil, ForkOutput{}, err
	}
	return nil, ForkOutput{
		ID:               result.id,
		Name:             result.name,
		ConnectionString: result.connectionString,
		Warnings:         result.warnings,
	}, nil
}

type forkDatabaseArgs struct {
	sourceDatabaseRef string
	req               api.ForkDatabaseRequest
	wait              bool
}

type forkDatabaseResult struct {
	id               string
	name             string
	size             string
	connectionString string
	warnings         []string
}

// forkDatabase is a shared helper for ghost_fork and ghost_fork_dedicated.
func (s *Server) forkDatabase(ctx context.Context, args forkDatabaseArgs) (forkDatabaseResult, error) {
	client, spaceID, err := s.app.GetClient()
	if err != nil {
		return forkDatabaseResult{}, err
	}

	// Fetch source database to check readiness
	sourceResp, err := client.GetDatabaseWithResponse(ctx, spaceID, args.sourceDatabaseRef)
	if err != nil {
		return forkDatabaseResult{}, fmt.Errorf("failed to get source database: %w", err)
	}
	if sourceResp.StatusCode() != http.StatusOK {
		return forkDatabaseResult{}, common.ExitWithErrorFromStatusCode(sourceResp.StatusCode(), sourceResp.JSONDefault)
	}
	if sourceResp.JSON200 == nil {
		return forkDatabaseResult{}, errors.New("empty response from API")
	}
	sourceDatabase := *sourceResp.JSON200

	// Check if the source database is ready
	if err := common.CheckReady(sourceDatabase); err != nil {
		return forkDatabaseResult{}, handleDatabaseError(err)
	}

	// Make API call to fork database
	// API defaults all other values based on Ghost space plan type
	forkResp, err := client.ForkDatabaseWithResponse(ctx, spaceID, args.sourceDatabaseRef, args.req)
	if err != nil {
		return forkDatabaseResult{}, fmt.Errorf("failed to fork database: %w", err)
	}

	// Handle API response
	if forkResp.StatusCode() != http.StatusAccepted {
		if common.IsNoPaymentMethod(forkResp.JSONDefault) && util.Deref(args.req.Type) == api.DatabaseTypeDedicated {
			return forkDatabaseResult{}, common.NoPaymentMethodError("fork a dedicated database")
		}
		return forkDatabaseResult{}, common.ExitWithErrorFromStatusCode(forkResp.StatusCode(), forkResp.JSONDefault)
	}

	if forkResp.JSON202 == nil {
		return forkDatabaseResult{}, errors.New("empty response from API")
	}
	database := *forkResp.JSON202
	databaseID := database.ID

	// Save password to .pgpass file
	var warnings []string
	password := util.Deref(database.Password)
	if password == "" {
		warnings = append(warnings, "no initial password returned by API")
	} else {
		if err := common.SavePassword(database, password, "tsdbadmin"); err != nil {
			warnings = append(warnings, fmt.Sprintf("failed to save password to .pgpass: %v", err))
		}
	}

	// Build connection string using InitialPassword directly
	connStr, err := common.BuildConnectionString(common.ConnectionStringArgs{
		Database: database,
		Role:     "tsdbadmin",
		Password: password,
	})
	if err != nil {
		warnings = append(warnings, fmt.Sprintf("failed to build connection string: %v", err))
	}

	if args.wait {
		if err := common.WaitForDatabase(ctx, common.WaitForDatabaseArgs{
			Client:      client,
			SpaceID:     spaceID,
			DatabaseRef: databaseID,
		}); err != nil {
			return forkDatabaseResult{}, err
		}
	}

	// TODO: use API response size when available instead of request size
	return forkDatabaseResult{
		id:               databaseID,
		name:             database.Name,
		size:             util.DerefStr(args.req.Size),
		connectionString: connStr,
		warnings:         warnings,
	}, nil
}
