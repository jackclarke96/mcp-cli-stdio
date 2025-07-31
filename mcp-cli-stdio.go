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

type Tool struct {
	Description string                 `json:"description"`
	Schema      map[string]interface{} `json:"inputSchema"`
}

var cachedTools = map[string]Tool{}

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

	fmt.Println("MCP CLI started. Type JSON-RPC messages / Ctrl + C to exit")

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

		if parsed == "" {
			// e.g., describe returned nothing to send
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

		// cache tools if it's a tools/list response
		if result, ok := pretty["result"]; ok {
			if tools, ok := result.(map[string]interface{})["tools"].([]interface{}); ok {
				cachedTools = map[string]Tool{}
				for _, t := range tools {
					toolObj := t.(map[string]interface{})
					name := toolObj["name"].(string)
					description := toolObj["description"].(string)
					if schema, ok := toolObj["inputSchema"].(map[string]interface{}); ok {
						cachedTools[name] = Tool{
							Description: description,
							Schema:      schema,
						}
					}
				}
				fmt.Printf("✅ Cached %d tool schemas\n", len(cachedTools))
			}
		}
	}
}

func parseLineToJSONRPC(line string) (string, error) {
	fields := strings.Fields(line)
	if len(fields) == 0 {
		return "", nil
	}

	switch fields[0] {
	case "list":
		if len(fields) > 1 && fields[1] == "--name-only" {
			if len(cachedTools) == 0 {
				return "", fmt.Errorf("tool cache is empty; run plain 'list' first")
			}
			fmt.Println("Available tools:")
			for name := range cachedTools {
				fmt.Printf("- %s\n", name)
			}
			return "", nil // Don't send anything to MCP
		}
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
				"name":      tool,
				"arguments": input,
			},
		}
		buf, err := json.Marshal(body)
		if err != nil {
			return "", err
		}
		return string(buf), nil

	default:
		if strings.HasPrefix(line, "call-") {
			toolName := strings.TrimPrefix(line, "call-")
			return promptForToolCall(toolName)
		}
		if strings.HasPrefix(line, "describe ") {
			toolName := strings.TrimPrefix(line, "describe ")
			return describeTool(toolName)
		}
		// Treat as raw JSON
		return line, nil
	}
}

func promptForToolCall(toolName string) (string, error) {
	tool, ok := cachedTools[toolName]
	if !ok {
		return "", fmt.Errorf("tool %q not found in cache; run 'list' first", toolName)
	}

	props := tool.Schema["properties"].(map[string]interface{})
	args := make(map[string]interface{})
	for key := range props {
		fmt.Printf("Enter value for %s: ", key)
		inputScanner := bufio.NewScanner(os.Stdin)
		inputScanner.Scan()
		val := inputScanner.Text()
		var parsed interface{}
		if err := json.Unmarshal([]byte(val), &parsed); err != nil {
			parsed = val // fallback to string
		}
		args[key] = parsed
	}

	payload := map[string]interface{}{
		"jsonrpc": "2.0",
		"method":  "tools/call",
		"id":      1,
		"params": map[string]interface{}{
			"name":      toolName,
			"arguments": args,
		},
	}

	buf, err := json.Marshal(payload)
	return string(buf), err
}

func describeTool(toolName string) (string, error) {
	tool, ok := cachedTools[toolName]
	if !ok {
		return "", fmt.Errorf("tool %q not found in cache; run 'list' first", toolName)
	}

	fmt.Printf("\n🔍 Tool: %s\n", toolName)
	fmt.Printf("📄 Description: %s\n\n", tool.Description)

	fmt.Println("📥 Input Schema:")
	describeProperties(tool.Schema, "", 1)
	fmt.Println()

	example := buildExampleFromTypes(tool.Schema)
	fmt.Println("Input Example:")
	fmt.Println(example)

	fmt.Println("Input Example JSON:")
	printExampleJSON(tool.Schema)

	return "", nil // don't send anything to server
}

func printExampleJSON(schema map[string]interface{}) {
	example := buildExampleFromSchema(schema, schema) // root = schema
	bytes, err := json.MarshalIndent(example, "", "  ")
	if err != nil {
		fmt.Println("❌ Failed to marshal:", err)
		return
	}
	fmt.Println("📦 Example request arguments:")
	fmt.Println(string(bytes))
}

// buildExampleFromSchema recursively builds a mock object based on the input schema.
func buildExampleFromSchema(schema map[string]interface{}, root map[string]interface{}) interface{} {
	if ref, ok := schema["$ref"].(string); ok {
		return buildExampleFromSchema(resolveRef(ref, root), root)
	}

	switch schema["type"] {
	case "object":
		obj := make(map[string]interface{})
		if props, ok := schema["properties"].(map[string]interface{}); ok {
			for key, val := range props {
				propSchema := val.(map[string]interface{})
				obj[key] = buildExampleFromSchema(propSchema, root)
			}
		}
		return obj
	case "array":
		if items, ok := schema["items"].(map[string]interface{}); ok {
			return []interface{}{buildExampleFromSchema(items, root)}
		}
		return []interface{}{}
	case "string":
		return "string"
	case "integer":
		return 0
	case "number":
		return 0.0
	case "boolean":
		if def, ok := schema["default"]; ok {
			return def
		}
		return false
	default:
		if def, ok := schema["default"]; ok {
			return def
		}
		return nil
	}
}

func describeProperties(schema map[string]interface{}, prefix string, depth int) {
	indent := strings.Repeat("  ", depth)
	props, ok := schema["properties"].(map[string]interface{})
	if !ok {
		return
	}

	for name, raw := range props {
		prop, _ := raw.(map[string]interface{})
		typ := prop["type"]
		desc := prop["description"]
		def := prop["default"]
		enum := prop["enum"]

		fieldPath := name
		if prefix != "" {
			fieldPath = prefix + "." + name
		}

		fmt.Printf("%s- %s (%v)", indent, fieldPath, typ)
		if desc != nil {
			fmt.Printf(": %v", desc)
		}
		fmt.Println()

		if def != nil {
			fmt.Printf("%s  ↳ default: %v\n", indent, def)
		}
		if enum != nil {
			fmt.Printf("%s  ↳ enum: %v\n", indent, enum)
		}

		// recurse if it's a nested object
		if typ == "object" {
			describeProperties(prop, fieldPath, depth+1)
		}
	}
}

func buildExampleFromTypes(schema map[string]interface{}) map[string]interface{} {
	return buildExampleTyped(schema, schema)
}

func buildExampleTyped(schema map[string]interface{}, root map[string]interface{}) map[string]interface{} {
	obj := map[string]interface{}{}
	props, _ := schema["properties"].(map[string]interface{})

	for name, raw := range props {
		prop := raw.(map[string]interface{})

		// Resolve $ref if needed
		if ref, ok := prop["$ref"].(string); ok {
			prop = resolveRef(ref, root)
		}

		typ, _ := prop["type"].(string)

		// Handle enums
		if enumList, ok := prop["enum"].([]interface{}); ok && len(enumList) > 0 {
			obj[name] = enumList[0]
			continue
		}

		switch typ {
		case "string":
			obj[name] = "string"
		case "integer":
			obj[name] = 0
		case "boolean":
			obj[name] = false
		case "array":
			items := prop["items"].(map[string]interface{})
			itemType := items["type"].(string)
			obj[name] = []interface{}{itemType}
		case "object":
			obj[name] = buildExampleTyped(prop, root)
		default:
			obj[name] = "any"
		}
	}

	return obj
}

func resolveRef(ref string, root map[string]interface{}) map[string]interface{} {
	if !strings.HasPrefix(ref, "#/") {
		fmt.Printf("⚠️ unsupported ref format: %s\n", ref)
		return map[string]interface{}{}
	}

	// Split path: e.g. "#/properties/value/properties/subject"
	path := strings.Split(ref[2:], "/")
	current := root
	for i, part := range path {
		unescaped := strings.ReplaceAll(part, "~1", "/")
		unescaped = strings.ReplaceAll(unescaped, "~0", "~")

		if next, ok := current[unescaped]; ok {
			// If we're not at the end, continue walking
			if m, ok := next.(map[string]interface{}); ok || i < len(path)-1 {
				current = m
			} else {
				return map[string]interface{}{}
			}
		} else {
			fmt.Printf("⚠️ ref path not found: %s (at %q)\n", ref, part)
			return map[string]interface{}{}
		}
	}

	return current
}
