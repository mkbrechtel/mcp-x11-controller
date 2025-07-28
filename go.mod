module mcp-x11-controller

go 1.23.0

toolchain go1.24.4

require (
	github.com/linuxdeepin/go-x11-client v0.0.0-00010101000000-000000000000
	github.com/modelcontextprotocol/go-sdk v0.2.0
)

require (
	github.com/stretchr/testify v1.10.0 // indirect
	github.com/yosida95/uritemplate/v3 v3.0.2 // indirect
	gopkg.in/check.v1 v1.0.0-20201130134442-10cb98267c6c // indirect
)

replace github.com/linuxdeepin/go-x11-client => /usr/share/gocode/src/github.com/linuxdeepin/go-x11-client
