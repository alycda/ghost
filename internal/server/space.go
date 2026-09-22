package server

import (
	"context"
	"time"

	"github.com/timescale/ghost/internal/api"
)

const apiKeyName = "ghost-server"

// keyPrefix identifies the API key without revealing it, the way the hosted
// service's gt_ prefixes did.
func (s *Server) keyPrefix() string {
	const n = 8
	if len(s.cfg.APIKey) <= n {
		return s.cfg.APIKey
	}
	return s.cfg.APIKey[:n]
}

func (s *Server) Health(context.Context, api.HealthRequestObject) (api.HealthResponseObject, error) {
	return api.Health200Response{}, nil
}

func (s *Server) AuthInfo(context.Context, api.AuthInfoRequestObject) (api.AuthInfoResponseObject, error) {
	return api.AuthInfo200JSONResponse(api.AuthInfo{
		Type: api.AuthInfoTypeAPIKey,
		APIKey: &api.APIKeyInfo{
			Prefix:    s.keyPrefix(),
			Name:      apiKeyName,
			CreatedAt: s.createdAt,
			SpaceID:   s.cfg.SpaceID,
			SpaceName: s.cfg.SpaceName,
			UserID:    s.cfg.SpaceID,
			UserEmail: s.cfg.UserEmail,
			UserName:  s.cfg.UserName,
		},
	}), nil
}

func (s *Server) ListSpaces(context.Context, api.ListSpacesRequestObject) (api.ListSpacesResponseObject, error) {
	return api.ListSpaces200JSONResponse{{ID: s.cfg.SpaceID, Name: s.cfg.SpaceName}}, nil
}

func (s *Server) GetSpace(_ context.Context, request api.GetSpaceRequestObject) (api.GetSpaceResponseObject, error) {
	if request.SpaceID != s.cfg.SpaceID {
		return api.GetSpacedefaultJSONResponse{Body: api.Error{Message: "space not found"}, StatusCode: 404}, nil
	}
	return api.GetSpace200JSONResponse(api.SpaceDetail{ID: s.cfg.SpaceID, Name: s.cfg.SpaceName}), nil
}

// usage reports real storage and zero compute: there is no meter here, and the
// limit is a number to display, not a quota that pauses anything.
func (s *Server) usage(ctx context.Context) (api.SpaceUsage, error) {
	bytes, err := s.totalStorageBytes(ctx)
	if err != nil {
		return api.SpaceUsage{}, err
	}
	now := time.Now().UTC()
	start := time.Date(now.Year(), now.Month(), 1, 0, 0, 0, 0, time.UTC)
	end := start.AddDate(0, 1, 0)
	zero := 0.0
	return api.SpaceUsage{
		StorageMib:         bytes / mib,
		StorageLimitMib:    s.cfg.StorageLimitMiB,
		ComputeMinutes:     0,
		FreeComputeMinutes: 0,
		OveragesEnabled:    false,
		CostToDate:         &zero,
		EstimatedTotalCost: &zero,
		BillingPeriodStart: &start,
		BillingPeriodEnd:   &end,
	}, nil
}

func (s *Server) SpaceUsage(ctx context.Context, request api.SpaceUsageRequestObject) (api.SpaceUsageResponseObject, error) {
	if request.SpaceID != s.cfg.SpaceID {
		return api.SpaceUsagedefaultJSONResponse{Body: api.Error{Message: "space not found"}, StatusCode: 404}, nil
	}
	u, err := s.usage(ctx)
	if err != nil {
		return nil, err
	}
	return api.SpaceUsage200JSONResponse(u), nil
}

func (s *Server) SpaceStatus(ctx context.Context, request api.SpaceStatusRequestObject) (api.SpaceStatusResponseObject, error) {
	if request.SpaceID != s.cfg.SpaceID {
		return api.SpaceStatusdefaultJSONResponse{Body: api.Error{Message: "space not found"}, StatusCode: 404}, nil
	}
	u, err := s.usage(ctx)
	if err != nil {
		return nil, err
	}
	return api.SpaceStatus200JSONResponse(u), nil
}

// ListAPIKeys shows the one key there is. Creating or deleting keys is not
// supported: the key is configuration, changed by restarting the server.
func (s *Server) ListAPIKeys(_ context.Context, request api.ListAPIKeysRequestObject) (api.ListAPIKeysResponseObject, error) {
	if request.SpaceID != s.cfg.SpaceID {
		return api.ListAPIKeysdefaultJSONResponse{Body: api.Error{Message: "space not found"}, StatusCode: 404}, nil
	}
	return api.ListAPIKeys200JSONResponse{{Prefix: s.keyPrefix(), Name: apiKeyName, CreatedAt: s.createdAt}}, nil
}
