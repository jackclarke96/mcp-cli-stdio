package main

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"syscall"

	"github.com/spf13/cobra"
)

func main() {
	var rootCmd = &cobra.Command{
		Use:   "mcp-cli",
		Short: "Interactive CLI for MCP over stdio",
		Run:   func(cmd *cobra.Command, args []string) { startInteractiveSession() },
	}

	if err := rootCmd.Execute(); err != nil {
		fmt.Println("Error:", err)
		os.Exit(1)
	}
}

func ensureFifo(path string) error {
	if _, err := os.Stat(path); os.IsNotExist(err) {
		err := syscall.Mkfifo(path, 0600)
		if err != nil {
			return fmt.Errorf("failed to create fifo %s: %w", path, err)
		}
	}
	return nil
}

func startInteractiveSession() {
	ensureFifo("mcp.stdin")
	ensureFifo("mcp.stdout")

	in, err := os.OpenFile("mcp.stdin", os.O_WRONLY, os.ModeNamedPipe)
	if err != nil {
		fmt.Println("Failed to open stdin:", err)
		return
	}
	defer in.Close()

	out, err := os.Open("mcp.stdout")
	if err != nil {
		fmt.Println("Failed to open stdout:", err)
		return
	}
	defer out.Close()

	inWriter := bufio.NewWriter(in)
	outReader := bufio.NewReader(out)
	inputScanner := bufio.NewScanner(os.Stdin)

	fmt.Println("MCP CLI started. Type JSON-RPC messages/ Ctrl + c to exit")

	for {
		fmt.Print("> ")
		if !inputScanner.Scan() {
			break
		}

		line := inputScanner.Text()
		if line == "" {
			continue
		}
		parsed, err := parseLineToJSONRPC(line)
		if err != nil {
			fmt.Println("❌ Error:", err)
			continue
		}
		var js map[string]interface{}
		if err := json.Unmarshal([]byte(parsed), &js); err != nil {
			fmt.Println("Invalid JSON:", err)
			continue
		}

		fmt.Println("sending parsed JSON:", parsed)
		_, err = inWriter.WriteString(parsed + "\n")
		if err != nil {
			fmt.Println("Error writing to MCP:", err)
			continue
		}
		inWriter.Flush()

		response, err := outReader.ReadBytes('\n')
		if err != nil {
			fmt.Println("Error reading from MCP:", err)
			continue
		}
		fmt.Println("Response:")
		var pretty map[string]interface{}
		json.Unmarshal(response, &pretty)
		prettyBytes, _ := json.MarshalIndent(pretty, "", "  ")
		fmt.Println(string(prettyBytes))
	}
}

func parseLineToJSONRPC(line string) (string, error) {
	fields := strings.Fields(line)
	if len(fields) == 0 {
		return "", nil
	}
	switch fields[0] {
	case "list":
		return `{"jsonrpc":"2.0","method":"tools/list","id":"1"}`, nil
	case "call":
		if len(fields) < 3 {
			return "", fmt.Errorf("usage: call <toolName> <json input>")
		}
		tool := fields[1]
		rest := strings.Join(fields[2:], " ")
		var input map[string]interface{}
		if err := json.Unmarshal([]byte(rest), &input); err != nil {
			return "", fmt.Errorf("invalid JSON input: %w", err)
		}
		body := map[string]interface{}{
			"jsonrpc": "2.0",
			"method":  "tools/call",
			"id":      "2",
			"params": map[string]interface{}{
				"tool":  tool,
				"input": input,
			},
		}
		buf, err := json.Marshal(body)
		if err != nil {
			return "", err
		}
		return string(buf), nil
	default:
		// Treat it as raw JSON
		return line, nil
	}
}
