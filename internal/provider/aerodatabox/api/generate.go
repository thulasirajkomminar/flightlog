//go:generate go run github.com/oapi-codegen/oapi-codegen/v2/cmd/oapi-codegen@v2.6.0 -generate types,client -package api -o client.gen.go openapi.yaml
package api
