package mcp

import (
	"bytes"
	"encoding/json"
	"errors"
	"strings"
	"testing"
)

func TestServe(t *testing.T) {
	s := Server{Name: "t", Version: "1", Tools: []Tool{
		{Name: "echo", Description: "echo", Call: func(a json.RawMessage) (string, error) {
			var p struct{ Text string }
			json.Unmarshal(a, &p)
			return p.Text, nil
		}},
		{Name: "fail", Call: func(json.RawMessage) (string, error) { return "", errors.New("nope") }},
	}}
	in := strings.Join([]string{
		`{"jsonrpc":"2.0","id":1,"method":"initialize","params":{"protocolVersion":"2025-06-18","capabilities":{},"clientInfo":{"name":"x","version":"0"}}}`,
		`{"jsonrpc":"2.0","method":"notifications/initialized"}`,
		`{"jsonrpc":"2.0","id":2,"method":"tools/list"}`,
		`{"jsonrpc":"2.0","id":"a","method":"tools/call","params":{"name":"echo","arguments":{"text":"hi"}}}`,
		`{"jsonrpc":"2.0","id":4,"method":"tools/call","params":{"name":"fail"}}`,
		`{"jsonrpc":"2.0","id":5,"method":"nope"}`,
		`not json`,
	}, "\n")
	var out bytes.Buffer
	if err := s.Serve(strings.NewReader(in), &out); err != nil {
		t.Fatal(err)
	}
	lines := strings.Split(strings.TrimSpace(out.String()), "\n")
	if len(lines) != 6 {
		t.Fatalf("want 6 responses, got %d:\n%s", len(lines), out.String())
	}
	for i, want := range []string{
		`"protocolVersion":"2025-06-18"`,
		`"name":"echo"`,
		`"id":"a","result":{"content":[{"text":"hi","type":"text"}]}`,
		`"isError":true`,
		`"code":-32601`,
		`"code":-32700`,
	} {
		if !strings.Contains(lines[i], want) {
			t.Errorf("response %d = %s, want it to contain %s", i, lines[i], want)
		}
	}
}
