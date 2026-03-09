#!/usr/bin/env node

import { StdioServerTransport } from "@modelcontextprotocol/sdk/server/stdio.js";
import { runLoginCli } from "./cli/login.js";
import { runRefreshCli } from "./cli/refresh.js";
import { createAudibleMcpServer } from "./mcp/server.js";

function parseArg(argv: string[], name: string): string | undefined {
  for (let index = 0; index < argv.length; index += 1) {
    const current = argv[index];
    const next = argv[index + 1];
    if (current === name) {
      return next;
    }
    if (current?.startsWith(`${name}=`)) {
      return current.slice(name.length + 1);
    }
  }
  return undefined;
}

function printUsage(): void {
  console.log("Usage:");
  console.log("  audible-mcp serve [--auth-file <path>] [--base-url <url>]");
  console.log("  audible-mcp auth login [--marketplace <code>] [--file <path>] [--no-open] [--serial <serial>] [--with-username]");
  console.log("  audible-mcp auth refresh [--file <path>] [--refresh-cookies]");
}

async function runServerCli(argv: string[]): Promise<void> {
  const authFile = parseArg(argv, "--auth-file") ?? process.env.AUDIBLE_AUTH_FILE;
  if (!authFile) {
    throw new Error("Provide an auth file with --auth-file or AUDIBLE_AUTH_FILE.");
  }

  const baseUrl = parseArg(argv, "--base-url") ?? process.env.AUDIBLE_BASE_URL;
  const server = await createAudibleMcpServer({
    authFile,
    ...(baseUrl !== undefined ? { baseUrl } : {}),
  });
  const transport = new StdioServerTransport();

  await server.connect(transport);
}

async function main(): Promise<void> {
  const [command, subcommand, ...rest] = process.argv.slice(2);

  if (command === undefined || command === "serve") {
    const serverArgv = command === "serve" ? [subcommand, ...rest].filter(Boolean) as string[] : process.argv.slice(2);
    await runServerCli(serverArgv);
    return;
  }

  if (command === "auth" && subcommand === "login") {
    await runLoginCli(rest);
    return;
  }

  if (command === "auth" && subcommand === "refresh") {
    await runRefreshCli(rest);
    return;
  }

  if (command === "help" || command === "--help" || command === "-h") {
    printUsage();
    return;
  }

  throw new Error(`Unknown command: ${[command, subcommand].filter(Boolean).join(" ")}`);
}

main().catch((error) => {
  const message = error instanceof Error ? error.message : String(error);
  console.error(message);
  printUsage();
  process.exitCode = 1;
});
