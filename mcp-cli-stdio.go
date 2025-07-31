package main

import (
	"bufio"
	"fmt"
	"os"
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
		fmt.Println("> ")
		if !inputScanner.Scan() {
			break
		}

		line := inputScanner.Text()
		if line == "" {
			continue
		}

		_, err := inWriter.WriteString(line + "\n")
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
		fmt.Println(string(response))
	}
}
