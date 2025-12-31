module gofast-mcp

go 1.25.5

require (
	github.com/conductorone/mcp-go-sdk v0.0.0
	github.com/google/pprof v0.0.0-20250820193118-f64d9cf942d6
	github.com/stretchr/testify v1.10.0
)

require (
	github.com/davecgh/go-spew v1.1.1 // indirect
	github.com/google/uuid v1.6.0 // indirect
	github.com/pmezard/go-difflib v1.0.0 // indirect
	gopkg.in/yaml.v3 v3.0.1 // indirect
)

replace github.com/conductorone/mcp-go-sdk => ../c1/local_vendor/mcp-go-sdk
