package server

import (
	"context"

	"github.com/timescale/ghost/internal/api"
)

const mib = 1024 * 1024

// wantsDedicated is a request for the hosted service's per-instance tier.
// Everything here is one cluster, so rather than hand over a standard
// database while the CLI prints the size it asked for, say no.
func wantsDedicated(t *api.DatabaseType, size *api.DatabaseSize) bool {
	return (t != nil && *t == api.DatabaseTypeDedicated) || (size != nil && *size != "")
}

// toAPI presents a record the way the CLI expects it: the connection details
// are the same for every database, only dbname and the label differ.
func (s *Server) toAPI(ctx context.Context, r record) (api.Database, error) {
	password, err := s.adminPassword(ctx)
	if err != nil {
		return api.Database{}, err
	}
	// Postgres is the truth about paused and running; the stored status
	// only matters while the database does not exist yet (configuring) or no
	// longer does (deleted). A resume whose bookkeeping failed, or a manual
	// ALLOW_CONNECTIONS, must not leave a connectable database that every
	// client refuses to touch.
	status := api.DatabaseStatus(r.Status)
	switch {
	case r.Exists && !r.AllowConn:
		status = api.DatabaseStatusPaused
	case r.Exists:
		status = api.DatabaseStatusRunning
	case status != api.DatabaseStatusConfiguring && status != api.DatabaseStatusDeleted:
		status = api.DatabaseStatusUnknown
	}
	dbname := r.ID
	db := api.Database{
		ID:       r.ID,
		Name:     r.Name,
		Type:     api.DatabaseTypeStandard,
		Status:   status,
		Host:     s.cfg.PublicHost,
		Port:     s.cfg.PublicPort,
		Dbname:   &dbname,
		Password: &password,
	}
	if r.SizeBytes != nil {
		size := *r.SizeBytes / mib
		db.StorageMib = &size
	}
	return db, nil
}

func (s *Server) ListDatabases(ctx context.Context, request api.ListDatabasesRequestObject) (api.ListDatabasesResponseObject, error) {
	records, err := s.list(ctx, request.SpaceID)
	if err != nil {
		body, status, err := failure(err)
		return api.ListDatabasesdefaultJSONResponse{Body: body, StatusCode: status}, err
	}
	out := make(api.ListDatabases200JSONResponse, 0, len(records))
	for _, r := range records {
		db, err := s.toAPI(ctx, r)
		if err != nil {
			return nil, err
		}
		out = append(out, db.WithUsage(nil))
	}
	return out, nil
}

func (s *Server) CreateDatabase(ctx context.Context, request api.CreateDatabaseRequestObject) (api.CreateDatabaseResponseObject, error) {
	if request.SpaceID != s.cfg.SpaceID {
		return api.CreateDatabasedefaultJSONResponse{Body: api.Error{Message: "space not found"}, StatusCode: 404}, nil
	}
	var name string
	if request.Body != nil {
		if request.Body.ShareToken != nil && *request.Body.ShareToken != "" {
			return api.CreateDatabasedefaultJSONResponse{Body: unsupported("creating from a share"), StatusCode: 501}, nil
		}
		if wantsDedicated(request.Body.Type, request.Body.Size) {
			return api.CreateDatabasedefaultJSONResponse{Body: unsupported("a dedicated database"), StatusCode: 501}, nil
		}
		if request.Body.Name != nil {
			name = *request.Body.Name
		}
	}
	r, err := s.create(ctx, name, nil)
	if err != nil {
		body, status, err := failure(err)
		return api.CreateDatabasedefaultJSONResponse{Body: body, StatusCode: status}, err
	}
	db, err := s.toAPI(ctx, r)
	if err != nil {
		return nil, err
	}
	return api.CreateDatabase202JSONResponse(db), nil
}

func (s *Server) GetDatabase(ctx context.Context, request api.GetDatabaseRequestObject) (api.GetDatabaseResponseObject, error) {
	r, err := s.resolve(ctx, request.SpaceID, request.DatabaseRef)
	if err != nil {
		body, status, err := failure(err)
		return api.GetDatabasedefaultJSONResponse{Body: body, StatusCode: status}, err
	}
	db, err := s.toAPI(ctx, r)
	if err != nil {
		return nil, err
	}
	return api.GetDatabase200JSONResponse(db), nil
}

func (s *Server) DeleteDatabase(ctx context.Context, request api.DeleteDatabaseRequestObject) (api.DeleteDatabaseResponseObject, error) {
	r, err := s.resolve(ctx, request.SpaceID, request.DatabaseRef)
	if err == nil {
		err = s.drop(ctx, r)
	}
	if err != nil {
		body, status, err := failure(err)
		return api.DeleteDatabasedefaultJSONResponse{Body: body, StatusCode: status}, err
	}
	r.Exists = false
	r.Status = string(api.DatabaseStatusDeleted)
	db, err := s.toAPI(ctx, r)
	if err != nil {
		return nil, err
	}
	db.Password = nil
	return api.DeleteDatabase202JSONResponse(db), nil
}

func (s *Server) ForkDatabase(ctx context.Context, request api.ForkDatabaseRequestObject) (api.ForkDatabaseResponseObject, error) {
	source, err := s.resolve(ctx, request.SpaceID, request.DatabaseRef)
	if err != nil {
		body, status, err := failure(err)
		return api.ForkDatabasedefaultJSONResponse{Body: body, StatusCode: status}, err
	}
	var name string
	if request.Body != nil {
		if wantsDedicated(request.Body.Type, request.Body.Size) {
			return api.ForkDatabasedefaultJSONResponse{Body: unsupported("a dedicated fork"), StatusCode: 501}, nil
		}
		if request.Body.Name != nil {
			name = *request.Body.Name
		}
	}
	if name == "" {
		name = source.Name + "-fork"
	}
	r, err := s.create(ctx, name, &source)
	if err != nil {
		body, status, err := failure(err)
		return api.ForkDatabasedefaultJSONResponse{Body: body, StatusCode: status}, err
	}
	db, err := s.toAPI(ctx, r)
	if err != nil {
		return nil, err
	}
	return api.ForkDatabase202JSONResponse(db), nil
}

func (s *Server) PauseDatabase(ctx context.Context, request api.PauseDatabaseRequestObject) (api.PauseDatabaseResponseObject, error) {
	r, err := s.resolve(ctx, request.SpaceID, request.DatabaseRef)
	if err == nil {
		err = s.setAllowConnections(ctx, r, false)
	}
	if err == nil {
		r, err = s.resolve(ctx, request.SpaceID, r.ID)
	}
	if err != nil {
		body, status, err := failure(err)
		return api.PauseDatabasedefaultJSONResponse{Body: body, StatusCode: status}, err
	}
	db, err := s.toAPI(ctx, r)
	if err != nil {
		return nil, err
	}
	return api.PauseDatabase202JSONResponse(db), nil
}

func (s *Server) ResumeDatabase(ctx context.Context, request api.ResumeDatabaseRequestObject) (api.ResumeDatabaseResponseObject, error) {
	r, err := s.resolve(ctx, request.SpaceID, request.DatabaseRef)
	if err == nil {
		err = s.setAllowConnections(ctx, r, true)
	}
	if err == nil {
		r, err = s.resolve(ctx, request.SpaceID, r.ID)
	}
	if err != nil {
		body, status, err := failure(err)
		return api.ResumeDatabasedefaultJSONResponse{Body: body, StatusCode: status}, err
	}
	db, err := s.toAPI(ctx, r)
	if err != nil {
		return nil, err
	}
	return api.ResumeDatabase202JSONResponse(db), nil
}

func (s *Server) RenameDatabase(ctx context.Context, request api.RenameDatabaseRequestObject) (api.RenameDatabaseResponseObject, error) {
	r, err := s.resolve(ctx, request.SpaceID, request.DatabaseRef)
	if err == nil {
		if request.Body == nil {
			err = badRequest("a name is required")
		} else {
			err = s.rename(ctx, r, request.Body.Name)
		}
	}
	if err != nil {
		body, status, err := failure(err)
		return api.RenameDatabasedefaultJSONResponse{Body: body, StatusCode: status}, err
	}
	return api.RenameDatabase204Response{}, nil
}

// UpdatePassword changes the password of the one shared role. The request
// names a database, as the API requires, but every database in the cluster is
// affected, because the CLI can only ever connect as tsdbadmin.
func (s *Server) UpdatePassword(ctx context.Context, request api.UpdatePasswordRequestObject) (api.UpdatePasswordResponseObject, error) {
	_, err := s.resolve(ctx, request.SpaceID, request.DatabaseRef)
	if err == nil {
		if request.Body == nil || request.Body.Password == "" {
			err = badRequest("a password is required")
		} else {
			err = s.setAdminPassword(ctx, request.Body.Password)
		}
	}
	if err != nil {
		body, status, err := failure(err)
		return api.UpdatePassworddefaultJSONResponse{Body: body, StatusCode: status}, err
	}
	return api.UpdatePassword204Response{}, nil
}

// DatabaseLogs has nothing to show yet: the cluster's log is one stream for all
// databases and this server does not read it. An empty page is the honest
// answer the CLI can display.
func (s *Server) DatabaseLogs(ctx context.Context, request api.DatabaseLogsRequestObject) (api.DatabaseLogsResponseObject, error) {
	if _, err := s.resolve(ctx, request.SpaceID, request.DatabaseRef); err != nil {
		body, status, err := failure(err)
		return api.DatabaseLogsdefaultJSONResponse{Body: body, StatusCode: status}, err
	}
	return api.DatabaseLogs200JSONResponse(api.LogsResponse{Entries: []api.LogEntry{}}), nil
}
