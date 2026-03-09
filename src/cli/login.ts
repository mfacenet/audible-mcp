import path from "node:path";
import { pathToFileURL } from "node:url";
import {
  createAudibleAuthFile,
  createExternalLoginSession,
  extractAuthorizationCode,
  registerAudibleDevice,
} from "../auth/audible-auth.js";
import { saveAuthFile } from "../auth/auth-file.js";
import { getMarketplace } from "../audible/marketplaces.js";
import { openBrowser } from "../util/browser.js";
import { prompt } from "../util/prompt.js";
import { readOptionValue } from "./args.js";

interface LoginCliOptions {
  file: string;
  marketplace: string;
  noOpen: boolean;
  serial?: string;
  withUsername: boolean;
}

function parseArgs(argv: string[]): LoginCliOptions {
  const options: LoginCliOptions = {
    file: "audible-auth.json",
    marketplace: "us",
    noOpen: false,
    withUsername: false,
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

    const marketplaceOption = readOptionValue(current, next, "--marketplace");
    if (marketplaceOption.value) {
      options.marketplace = marketplaceOption.value;
      if (marketplaceOption.consumedNext) {
        index += 1;
      }
      continue;
    }

    const localeOption = readOptionValue(current, next, "--locale");
    if (localeOption.value) {
      options.marketplace = localeOption.value;
      if (localeOption.consumedNext) {
        index += 1;
      }
      continue;
    }

    const serialOption = readOptionValue(current, next, "--serial");
    if (serialOption.value) {
      options.serial = serialOption.value;
      if (serialOption.consumedNext) {
        index += 1;
      }
      continue;
    }

    if (current === "--with-username") {
      options.withUsername = true;
      continue;
    }

    if (current === "--no-open") {
      options.noOpen = true;
    }
  }

  return options;
}

export async function runLoginCli(argv: string[] = process.argv.slice(2)): Promise<void> {
  const options = parseArgs(argv);
  const marketplace = getMarketplace(options.marketplace);
  const session = createExternalLoginSession(marketplace, {
    withUsername: options.withUsername,
    ...(options.serial ? { serial: options.serial } : {}),
  });

  console.log(`Marketplace: ${marketplace.code} (${marketplace.domain})`);
  console.log(`Device serial: ${session.serial}`);
  console.log("");
  console.log("Open this URL in a browser and complete the Audible/Amazon login flow:");
  console.log(session.loginUrl);
  console.log("");

  if (!options.noOpen) {
    await openBrowser(session.loginUrl);
    console.log("Attempted to open the browser automatically.");
    console.log("");
  }

  console.log("After login, copy the final URL from the browser address bar.");
  const responseUrl = await prompt("Paste the final maplanding URL: ");
  const authorizationCode = extractAuthorizationCode(responseUrl);
  const registration = await registerAudibleDevice(session, authorizationCode);
  const authFile = createAudibleAuthFile(session, registration);

  await saveAuthFile(options.file, authFile);

  console.log("");
  console.log(`Saved auth bundle to ${path.resolve(options.file)}`);
  console.log("This registration creates a virtual Audible device. Reuse this file instead of registering repeatedly.");
}

const isDirectExecution =
  process.argv[1] !== undefined && import.meta.url === pathToFileURL(process.argv[1]).href;

if (isDirectExecution) {
  runLoginCli().catch((error) => {
    const message = error instanceof Error ? error.message : String(error);
    console.error(message);
    process.exitCode = 1;
  });
}
