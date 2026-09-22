package api

//go:generate go tool oapi-codegen -config types.yaml ../../openapi.yaml
//go:generate go tool oapi-codegen -config client.yaml ../../openapi.yaml
//go:generate go tool oapi-codegen -config server.yaml ../../openapi.yaml
//go:generate go tool mockgen -source=client.go -destination=mock/mock_client.go -package=mock ClientWithResponsesInterface
