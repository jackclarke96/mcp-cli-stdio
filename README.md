# 🧠 MCP CLI (Interactive)

An interactive terminal interface for sending JSON-RPC messages to an MCP server
over stdio — works with local and Docker-based MCP servers.

## ✅ Features

- 🧠 JSON-RPC REPL with arrow-key history
- 🚀 Auto-launches a child process for the MCP server
- 🧼 Clean shutdown with Ctrl + C
- 🔎 Tool discovery and schema caching
- 🐳 Docker-compatible with live stdin/stdout piping

---

## 📦 Installation

Ensure you have:

- Go 1.18+
- Docker (if using Docker images)
- A compiled MCP server or a public Docker image like

---

## 🧪 Usage

### 🔁 Start with a local Node.js server

```bash
go run mcp-cli-stdio.go \
  --start-cmd "cd ../mcp-local && node dist/index.js -e .env"
```

### 🐳 Start with Docker

```bash
go run mcp-cli-stdio.go \
  --start-cmd "docker run -i --rm \
    -e MY_ENV_VAR="value" \
   docker-mcp:latest --transport stdio"
```

> ✅ Make sure to use `-i` so Docker keeps stdin open.

---

## 💬 Commands

Once launched, you’ll see:

```
MCP CLI started. Type JSON-RPC messages / Ctrl + C to exit
>
```

You can now type:

- `list` — calls `tools/list` method and caches tool metadata
- `list --name-only` — shows available tool names (after `list`)
- `describe <tool_name>` - gives details about tool schema and how to call, as
  well as a basic JSON object to build your response from
- `call <tool_name> <JSON>` calls the passed tool with the passed JSON
- Any valid JSON-RPC message, e.g.:
  ```json
  { "jsonrpc": "2.0", "method": "list-topics", "params": {}, "id": "2" }
  ```

---

## 🧹 Cleanup

Hitting `Ctrl + C` will automatically kill the child process (Docker or Node)
and exit cleanly.

---
