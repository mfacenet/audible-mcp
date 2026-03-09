import path from "node:path";
import { pathToFileURL } from "node:url";
import { refreshAudibleAccessToken, refreshAudibleWebsiteCookies } from "../auth/audible-auth.js";
import { loadAuthFile, saveAuthFile } from "../auth/auth-file.js";
import { readOptionValue } from "./args.js";

interface RefreshCliOptions {
  file: string;
  refreshCookies: boolean;
}

function parseArgs(argv: string[]): RefreshCliOptions {
  const options: RefreshCliOptions = {
    file: "audible-auth.json",
    refreshCookies: false,
  };

  for (let index = 0; index < argv.length; index += 1) {
    const current = argv[index];
    const next = argv[index + 1];
    if (current === undefined) {
      continue;
    }

    const fileOption = readOptionValue(current, next, "--file");
    if (fileOption.value) {
      options.file = fileOption.value;
      if (fileOption.consumedNext) {
        index += 1;
      }
      continue;
    }

    if (current === "--refresh-cookies") {
      options.refreshCookies = true;
    }
  }

  return options;
}

export async function runRefreshCli(argv: string[] = process.argv.slice(2)): Promise<void> {
  const options = parseArgs(argv);
  const authFile = await loadAuthFile(options.file);
  const refreshed = await refreshAudibleAccessToken(authFile);

  authFile.accessToken = refreshed.accessToken;
  authFile.expiresAt = refreshed.expiresAt;

  if (options.refreshCookies) {
    authFile.websiteCookies = await refreshAudibleWebsiteCookies(authFile, authFile.domain);
  }

  await saveAuthFile(options.file, authFile);

  console.log(`Updated auth bundle at ${path.resolve(options.file)}`);
}

const isDirectExecution =
  process.argv[1] !== undefined && import.meta.url === pathToFileURL(process.argv[1]).href;

if (isDirectExecution) {
  runRefreshCli().catch((error) => {
    const message = error instanceof Error ? error.message : String(error);
    console.error(message);
    process.exitCode = 1;
  });
}
