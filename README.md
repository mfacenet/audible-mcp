# audible-mcp

[![CI](https://github.com/mfacenet/audible-mcp/actions/workflows/ci.yml/badge.svg?branch=main)](https://github.com/mfacenet/audible-mcp/actions/workflows/ci.yml)

Go Model Context Protocol server for authenticated Audible **read** workflows: library, wishlist, collections, catalog metadata, and listening stats.

This is a personal fork and Go rewrite of [tannerwj/audible-mcp](https://github.com/tannerwj/audible-mcp). It is **not affiliated with Audible or Amazon**.

## Requirements

- Go 1.27+
- an `audible-auth.json` bundle from `audible-mcp auth login` (or a bundle created by the original TypeScript CLI)

## Setup

```sh
go build -o bin/audible-mcp ./cmd/audible-mcp
./bin/audible-mcp auth login --marketplace us --file ./audible-auth.json
```

The login flow opens Amazon/Audible in your browser. Paste the final `maplanding` URL back into the terminal. The resulting `audible-auth.json` contains live credentials and must not be committed.

Refresh without re-registering the device:

```sh
./bin/audible-mcp auth refresh --file ./audible-auth.json
```

## Run the MCP server

```sh
AUDIBLE_AUTH_FILE=./audible-auth.json ./bin/audible-mcp serve
```

## Example MCP config

```json
{
  "mcpServers": {
    "audible": {
      "command": "/path/to/audible-mcp",
      "args": ["serve"],
      "env": {
        "AUDIBLE_AUTH_FILE": "/path/to/audible-auth.json"
      }
    }
  }
}
```

`mcp.config.example.json` is the committed template. Copy it to `mcp.config.json` (gitignored) for local use.

## Tools

- `audible_list_library`
- `audible_list_collection_items`
- `audible_search_library`
- `audible_list_in_progress_titles`
- `audible_get_library_item`
- `audible_list_wishlist`
- `audible_list_collections`
- `audible_get_chapters`
- `audible_get_content_metadata`
- `audible_get_catalog_product`
- `audible_get_listening_stats`
- `audible_validate_auth`
- `audible_get_auth_status`

## Resources

- `audible://auth/status`
- `audible://wishlist`
- `audible://collections`
- `audible://collections/{collectionId}/items`
- `audible://library/{asin}`
- `audible://content/{asin}/metadata`
- `audible://catalog/{asin}`

## Development

```sh
task test
task test:race
task build
```

Or without Task: `go test ./...` and `go build -o bin/audible-mcp ./cmd/audible-mcp`.

## License

MIT. See [LICENSE](LICENSE) and [NOTICE](NOTICE).

Auth talks to Amazon's unofficial Audible iOS API for **your own account**. Keep the server read-only. Do not commit `audible-auth.json`.
