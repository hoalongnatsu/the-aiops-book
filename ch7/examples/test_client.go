package main

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os/exec"
)

type JSONRPCRequest struct {
	JSONRPC string      `json:"jsonrpc"`
	ID      int         `json:"id"`
	Method  string      `json:"method"`
	Params  interface{} `json:"params,omitempty"`
}

type JSONRPCResponse struct {
	JSONRPC string      `json:"jsonrpc"`
	ID      int         `json:"id"`
	Result  interface{} `json:"result,omitempty"`
	Error   interface{} `json:"error,omitempty"`
}

func main() {
	// Start the MCP server
	cmd := exec.Command("./bin/aws-mcp-server")

	stdin, err := cmd.StdinPipe()
	if err != nil {
		panic(err)
	}

	stdout, err := cmd.StdoutPipe()
	if err != nil {
		panic(err)
	}

	if err := cmd.Start(); err != nil {
		panic(err)
	}

	// Test initialization
	initReq := JSONRPCRequest{
		JSONRPC: "2.0",
		ID:      1,
		Method:  "initialize",
		Params:  map[string]interface{}{},
	}

	reqData, _ := json.Marshal(initReq)
	fmt.Fprintf(stdin, "%s\n", reqData)

	// Read response
	scanner := bufio.NewScanner(stdout)
	if scanner.Scan() {
		var resp JSONRPCResponse
		json.Unmarshal(scanner.Bytes(), &resp)
		fmt.Printf("Initialize response: %+v\n", resp)
	}

	// Test resource listing
	listReq := JSONRPCRequest{
		JSONRPC: "2.0",
		ID:      2,
		Method:  "resources/list",
	}

	reqData, _ = json.Marshal(listReq)
	fmt.Fprintf(stdin, "%s\n", reqData)

	if scanner.Scan() {
		var resp JSONRPCResponse
		json.Unmarshal(scanner.Bytes(), &resp)
		fmt.Printf("Resources list: %+v\n", resp)
	}

	stdin.Close()
	cmd.Wait()
}
