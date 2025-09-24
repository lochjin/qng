package mcp

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/Qitmeer/qng/core/blockchain"
	mcpgo "github.com/mark3labs/mcp-go/mcp"
)

func (m *MCPService) handleTips(ctx context.Context, request mcpgo.CallToolRequest) (*mcpgo.CallToolResult, error) {
	if m.rpcSer.BC == nil {
		return nil, fmt.Errorf("rpc no init")
	}
	blockAPI, ok := getAPI[*blockchain.PublicBlockAPI](m.apis)
	if !ok {
		return nil, fmt.Errorf("rpc api error")
	}
	ret, err := blockAPI.Tips()
	if err != nil {
		return nil, err
	}
	jsonStr, err := json.Marshal(ret)
	if err != nil {
		return nil, err
	}
	return mcpgo.NewToolResultText(string(jsonStr)), nil
}

func (m *MCPService) registerTools() error {
	if m.rpcSer == nil {
		return fmt.Errorf("rpc server not initialized")
	}

	return nil
}
