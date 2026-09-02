# audible-mcp

[![CI](https://github.com/mfacenet/audible-mcp/actions/workflows/ci.yml/badge.svg?branch=main)](https://github.com/mfacenet/audible-mcp/actions/workflows/ci.yml)

Go Model Context Protocol server for authenticated Audible **read** workflows: library, wishlist, collections, catalog metadata, and listening stats.

This is a personal fork and Go rewrite of [tannerwj/audible-mcp](https://github.com/tannerwj/audible-mcp). It is **not affiliated with Audible or Amazon**.

## Install

### Go

```sh
go install github.com/mfacenet/audible-mcp/cmd/audible-mcp@latest
```

Pin a release with `@v2.0.0` once that tag exists. The binary lands on `$GOBIN` (usually `$HOME/go/bin`).

### GitHub Releases

Download the archive for your OS/arch from [Releases](https://github.com/mfacenet/audible-mcp/releases), unpack `audible-mcp`, and put it on your `PATH`.

### Docker

Images are `ghcr.io/mfacenet/audible-mcp`. Bind-mount the auth file; do not bake credentials into the image.

```sh
docker pull ghcr.io/mfacenet/audible-mcp:latest
```

## Setup

You need an `audible-auth.json` bundle before serving. Existing bundles from the original TypeScript CLI still work.

```sh
audible-mcp auth login --marketplace us --file ./audible-auth.json
```

The login flow opens Amazon/Audible in your browser. Paste the final `maplanding` URL back into the terminal. The resulting file contains live credentials and must not be committed.

Refresh without re-registering the device:

```sh
audible-mcp auth refresh --file ./audible-auth.json
```

## Run the MCP server

```sh
AUDIBLE_AUTH_FILE=./audible-auth.json audible-mcp serve
```

## Example MCP config

Binary on `PATH`:

```json
{
  "mcpServers": {
    "audible": {
      "command": "audible-mcp",
      "args": ["serve"],
      "env": {
        "AUDIBLE_AUTH_FILE": "/path/to/audible-auth.json"
      }
    }
  }
}
```

Docker (stdio). The auth file is mounted read-only:

```json
{
  "mcpServers": {
    "audible": {
      "command": "docker",
      "args": [
        "run",
        "-i",
        "--rm",
        "-v",
        "/path/to/audible-auth.json:/auth/audible-auth.json:ro",
        "-e",
        "AUDIBLE_AUTH_FILE=/auth/audible-auth.json",
        "ghcr.io/mfacenet/audible-mcp"
      ]
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
task release:snapshot
```

Or without Task: `go test ./...` and `go build -o bin/audible-mcp ./cmd/audible-mcp`.

Release process: [docs/release.md](docs/release.md).

## License

MIT. See [LICENSE](LICENSE) and [NOTICE](NOTICE).

Auth talks to Amazon's unofficial Audible iOS API for **your own account**. Keep the server read-only. Do not commit `audible-auth.json`.
