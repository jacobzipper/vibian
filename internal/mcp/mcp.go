// Package mcp is a minimal Model Context Protocol server over stdio
// (newline-delimited JSON-RPC 2.0): initialize, tools/list, tools/call, ping.
// It is all Vibian's MCP servers need, without an SDK dependency.
package mcp

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"slices"
)

// Tool is one callable tool. Schema is its JSON Schema for arguments.
type Tool struct {
	Name        string
	Description string
	Schema      map[string]any
	ReadOnly    bool
	Call        func(args json.RawMessage) (string, error)
}

// Server answers MCP requests for a fixed set of tools.
type Server struct {
	Name, Version, Instructions string
	Tools                       []Tool
}

var versions = []string{"2025-11-25", "2025-06-18", "2025-03-26", "2024-11-05"}

type request struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      json.RawMessage `json:"id,omitempty"`
	Method  string          `json:"method"`
	Params  json.RawMessage `json:"params,omitempty"`
}

type rpcError struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}

type response struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      json.RawMessage `json:"id"`
	Result  any             `json:"result,omitempty"`
	Error   *rpcError       `json:"error,omitempty"`
}

// Serve handles requests from r until EOF, writing responses to w.
func (s Server) Serve(r io.Reader, w io.Writer) error {
	in := bufio.NewScanner(r)
	in.Buffer(make([]byte, 1<<20), 16<<20)
	enc := json.NewEncoder(w)
	for in.Scan() {
		line := in.Bytes()
		if len(line) == 0 {
			continue
		}
		var req request
		if err := json.Unmarshal(line, &req); err != nil {
			enc.Encode(response{JSONRPC: "2.0", ID: json.RawMessage("null"), Error: &rpcError{-32700, "parse error"}})
			continue
		}
		if len(req.ID) == 0 {
			continue // a notification (initialized, cancelled): nothing to answer
		}
		res, rerr := s.handle(req)
		out := response{JSONRPC: "2.0", ID: req.ID, Result: res, Error: rerr}
		if err := enc.Encode(out); err != nil {
			return err
		}
	}
	return in.Err()
}

func (s Server) handle(req request) (any, *rpcError) {
	switch req.Method {
	case "initialize":
		var p struct {
			ProtocolVersion string `json:"protocolVersion"`
		}
		json.Unmarshal(req.Params, &p)
		v := versions[0]
		if slices.Contains(versions, p.ProtocolVersion) {
			v = p.ProtocolVersion
		}
		return map[string]any{
			"protocolVersion": v,
			"capabilities":    map[string]any{"tools": map[string]any{}},
			"serverInfo":      map[string]any{"name": s.Name, "version": s.Version},
			"instructions":    s.Instructions,
		}, nil
	case "ping":
		return map[string]any{}, nil
	case "tools/list":
		list := []map[string]any{}
		for _, t := range s.Tools {
			schema := t.Schema
			if schema == nil {
				schema = map[string]any{"type": "object", "properties": map[string]any{}}
			}
			list = append(list, map[string]any{
				"name": t.Name, "description": t.Description, "inputSchema": schema,
				"annotations": map[string]any{"readOnlyHint": t.ReadOnly},
			})
		}
		return map[string]any{"tools": list}, nil
	case "tools/call":
		var p struct {
			Name      string          `json:"name"`
			Arguments json.RawMessage `json:"arguments"`
		}
		if err := json.Unmarshal(req.Params, &p); err != nil {
			return nil, &rpcError{-32602, "bad params"}
		}
		for _, t := range s.Tools {
			if t.Name != p.Name {
				continue
			}
			if len(p.Arguments) == 0 {
				p.Arguments = json.RawMessage("{}")
			}
			text, err := t.Call(p.Arguments)
			if err != nil {
				// tool errors go back to the model as a result, so it can react
				return map[string]any{"content": []map[string]any{{"type": "text", "text": err.Error()}}, "isError": true}, nil
			}
			return map[string]any{"content": []map[string]any{{"type": "text", "text": text}}}, nil
		}
		return nil, &rpcError{-32602, fmt.Sprintf("unknown tool %q", p.Name)}
	}
	return nil, &rpcError{-32601, "method not found: " + req.Method}
}
