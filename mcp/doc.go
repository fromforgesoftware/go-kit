// Package mcp is a thin idiomatic Go wrapper around the mark3labs MCP
// SDK with forge-style typed tool registration: an MCP tool is defined
// by a Go struct (input) + Go struct (output) and a typed handler;
// the JSON Schema exposed to clients is generated from the input
// struct via reflection.
//
// Typical use:
//
//	srv := mcp.New(mcp.Config{Name: "my-server", Version: "0.1"})
//	mcp.Register(srv, mcp.Tool[EchoIn, EchoOut]{
//	    Name:        "echo",
//	    Description: "Echoes its argument back.",
//	    Handler:     func(ctx context.Context, in EchoIn) (EchoOut, error) { ... },
//	})
//	_ = srv.Run(ctx)
//
// Transports: stdio (default), HTTP. The package never imports
// kit/unreal/* or any project service so unrelated MCP projects
// (trading-bot, ops MCPs, unreal-mcp) can all share the harness.
package mcp
