# 1. Implement audible-mcp in Go under MIT

- **Status**: accepted
- **Date**: 2026-09-01
- **Deciders**: Shawn Stratton

## Context and Problem Statement

`mfacenet/audible-mcp` is a personal fork of tannerwj/audible-mcp, a
TypeScript MCP server for authenticated Audible library reads. The fork
needed a default `main` branch, a language that matches how the fork
will be maintained, and a license that is not AGPL.

The TypeScript auth helpers closely follow mkb79/Audible (AGPL-3.0).
Keeping that tree and claiming MIT is a provenance risk. The fork is
five commits old, so a rewrite is still cheap.

## Decision Drivers

- Personal fork, not an npm package to keep in lockstep with upstream
- Single static binary is a better MCP install than Node 22 + npx
- Official Go MCP SDK (`github.com/modelcontextprotocol/go-sdk` v1.7.0)
- Avoid AGPL copyleft on the auth path
- Preserve audible-auth.json and the existing tool/resource names

## Considered Options

- Keep TypeScript and MIT as published by tannerwj
- Keep TypeScript and relicense the combined work AGPL-3.0
- Rewrite in Go under MIT, implementing Amazon's iOS protocol directly
- Rewrite in Rust, following mkb79/audible-rs

## Decision Outcome

Chosen option: "Rewrite in Go under MIT", because the fork will diverge,
Go is the maintainer's primary toolchain, the official MCP SDK is
current, and a protocol-level rewrite removes the AGPL-shaped TypeScript
auth port.

### Consequences

- **Positive**: one binary; stdlib crypto; MIT; auth file compatibility
- **Negative**: no longer a drop-in npm/`npx` package; cannot contribute
  TypeScript PRs back to tannerwj without a separate checkout
- **Neutral**: MCP tool and resource names stay the same

## Pros and Cons of the Options

### Keep TypeScript and MIT

- Good, because the code already works and MCP clients already know npx
- Bad, because auth looks like a translation of AGPL Python and the
  maintainer does not want to live in that gray area

### Keep TypeScript and AGPL the fork

- Good, because it is the conservative reading of the auth provenance
- Bad, because AGPL is a poor fit for a personal MCP the maintainer may
  reuse, and the user explicitly rejected AGPL

### Rewrite in Go under MIT

- Good, because it matches the maintainer's stack and produces a single
  binary, with MIT on new code plus tannerwj's MIT notice for the
  original server contract
- Bad, because it is a one-way rewrite and distribution moves off npm

### Rewrite in Rust

- Good, because mkb79/audible-rs is MIT and already covers this protocol
- Bad, because this repo is an MCP server, not an Audible downloader,
  and Go already has a first-party MCP SDK

## Links

- Original TypeScript server: https://github.com/tannerwj/audible-mcp
- Official Go MCP SDK: https://github.com/modelcontextprotocol/go-sdk
- Protocol prior art (MIT): https://github.com/mkb79/audible-rs
- AGPL Python library (not used as source): https://github.com/mkb79/Audible
