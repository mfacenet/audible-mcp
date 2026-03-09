# audible-mcp

`audible-mcp` is a TypeScript Model Context Protocol server for authenticated Audible read workflows.

It currently exposes signed-auth tools and resources for:

- library listing
- collection item listing
- library search
- in-progress title listing
- single library item lookup
- wishlist listing
- collections listing
- chapter lookup by ASIN
- content metadata by ASIN
- catalog product metadata by ASIN
- aggregate listening stats
- auth validation
- local auth status

## Requirements

- Node.js 22+
- an Audible auth bundle created with the login flow in this repo or via the published CLI

## Setup

Install dependencies:

```powershell
npm install
```

Create a local Audible auth bundle:

```powershell
npm run auth:login -- --marketplace us --file .\audible-auth.json
```

The login flow opens Audible/Amazon in your browser, then asks you to paste the final `maplanding` URL back into the terminal. The resulting `audible-auth.json` contains live credentials and is gitignored.

Refresh the auth bundle later without re-registering the device:

```powershell
npm run auth:refresh -- --file .\audible-auth.json
```

The published package uses the same CLI shape through `npx`:

```sh
npx audible-mcp auth login --marketplace us --file ./audible-auth.json
npx audible-mcp auth refresh --file ./audible-auth.json
```

## Run The MCP Server

Start the stdio server directly:

```powershell
$env:AUDIBLE_AUTH_FILE = ".\audible-auth.json"
npm run mcp:start
```

After npm publish, the equivalent command is:

```sh
AUDIBLE_AUTH_FILE=./audible-auth.json npx audible-mcp serve
```

Run the end-to-end MCP smoke test:

```powershell
$env:AUDIBLE_AUTH_FILE = ".\audible-auth.json"
npm run smoke:test
```

## Root Config Files

- `mcp.config.json`: local MCP client config for this checkout; gitignored
- `mcp.config.example.json`: committed template you can copy and edit for your machine

The local config uses the server command from this repo and points `AUDIBLE_AUTH_FILE` at the root auth bundle.

## Example MCP Config

The example config uses the common `mcpServers` shape:

```json
{
  "mcpServers": {
    "audible": {
      "command": "npm",
      "args": ["run", "mcp:start"],
      "cwd": "/path/to/audible-mcp",
      "env": {
        "AUDIBLE_AUTH_FILE": "/path/to/audible-mcp/audible-auth.json"
      }
    }
  }
}
```

The committed template lives in `mcp.config.example.json`.

## Available Tools

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

## Available Resources

- `audible://auth/status`
- `audible://wishlist`
- `audible://collections`
- `audible://collections/{collectionId}/items`
- `audible://library/{asin}`
- `audible://content/{asin}/metadata`
- `audible://catalog/{asin}`

## Development Scripts

- `npm run auth:login`
- `npm run auth:refresh`
- `npm run mcp:start`
- `npm run smoke:test`
- `npm run typecheck`
- `npm run test`
- `npm run build`
- `npm run check`

## Notes

- Signed auth is the working API path in this repo.
- `audible-auth.json` contains private credentials and should be treated as a secret.
- The main CLI supports `serve`, `auth login`, and `auth refresh`.
- CI runs `npm run check` on pushes to `master` and on pull requests.

## Security

- Do not commit `audible-auth.json`.
- If the auth bundle is ever exposed, revoke or replace it by generating a new device registration.
- `mcp.config.json` is local-only and should stay uncommitted.
