package mcp_test

import (
	"context"
	"testing"

	mcpclient "github.com/mark3labs/mcp-go/client"
	mcpgo "github.com/mark3labs/mcp-go/mcp"

	"github.com/fromforgesoftware/go-kit/mcp"
)

type benchIn struct {
	A int `json:"a"`
	B int `json:"b"`
}

type benchOut struct {
	Sum int `json:"sum"`
}

func BenchmarkSchemaForFlatStruct(b *testing.B) {
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = mcp.SchemaFor[benchIn]()
	}
}

type bigSchema struct {
	A string  `json:"a"`
	B string  `json:"b"`
	C string  `json:"c"`
	D string  `json:"d"`
	E string  `json:"e"`
	F int     `json:"f"`
	G int     `json:"g"`
	H int     `json:"h"`
	I int     `json:"i"`
	J int     `json:"j"`
	K float64 `json:"k"`
	L float64 `json:"l"`
	M float64 `json:"m"`
	N float64 `json:"n"`
	O float64 `json:"o"`
	P bool    `json:"p"`
	Q bool    `json:"q"`
	R bool    `json:"r"`
	S bool    `json:"s"`
	T bool    `json:"t"`
}

func BenchmarkSchemaForLargeStruct(b *testing.B) {
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = mcp.SchemaFor[bigSchema]()
	}
}

func BenchmarkToolCallInProcess(b *testing.B) {
	s := mcp.New(mcp.Config{Name: "bench"})
	mcp.Register(s, mcp.Tool[benchIn, benchOut]{
		Name:        "add",
		Description: "add two ints",
		Handler: func(_ context.Context, in benchIn) (benchOut, error) {
			return benchOut{Sum: in.A + in.B}, nil
		},
	})
	c, err := mcpclient.NewInProcessClient(s.MCP())
	if err != nil {
		b.Fatal(err)
	}
	defer c.Close()
	if err := c.Start(context.Background()); err != nil {
		b.Fatal(err)
	}
	initReq := mcpgo.InitializeRequest{}
	initReq.Params.ProtocolVersion = mcpgo.LATEST_PROTOCOL_VERSION
	initReq.Params.ClientInfo = mcpgo.Implementation{Name: "bench", Version: "0"}
	if _, err := c.Initialize(context.Background(), initReq); err != nil {
		b.Fatal(err)
	}

	req := mcpgo.CallToolRequest{}
	req.Params.Name = "add"
	req.Params.Arguments = map[string]any{"a": 1, "b": 2}
	ctx := context.Background()
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if _, err := c.CallTool(ctx, req); err != nil {
			b.Fatal(err)
		}
	}
}
