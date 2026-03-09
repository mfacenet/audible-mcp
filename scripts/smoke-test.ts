import { Client } from "@modelcontextprotocol/sdk/client/index.js";
import { StdioClientTransport } from "@modelcontextprotocol/sdk/client/stdio.js";
import path from "node:path";

type JsonRecord = Record<string, unknown>;

const EXPECTED_TOOLS = [
  "audible_list_library",
  "audible_get_library_item",
  "audible_list_wishlist",
  "audible_list_collections",
  "audible_list_collection_items",
  "audible_search_library",
  "audible_list_in_progress_titles",
  "audible_get_content_metadata",
  "audible_get_chapters",
  "audible_get_catalog_product",
  "audible_get_listening_stats",
  "audible_validate_auth",
  "audible_get_auth_status",
] as const;

const EXPECTED_RESOURCES = [
  "audible://auth/status",
  "audible://wishlist",
  "audible://collections",
] as const;

const EXPECTED_RESOURCE_TEMPLATES = [
  "audible://library/{asin}",
  "audible://collections/{collectionId}/items",
  "audible://content/{asin}/metadata",
  "audible://catalog/{asin}",
] as const;

function parseArg(name: string): string | undefined {
  const argv = process.argv.slice(2);
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

function parseToolResult(result: unknown): JsonRecord {
  const record = result !== null && typeof result === "object" ? (result as JsonRecord) : {};
  const content = Array.isArray(record.content)
    ? (record.content as Array<{ type?: string; text?: string | undefined }>)
    : undefined;
  const structuredContent =
    record.structuredContent !== null && typeof record.structuredContent === "object"
      ? (record.structuredContent as JsonRecord)
      : undefined;

  if (record.isError === true) {
    const text = content?.find((entry) => entry.type === "text")?.text;
    throw new Error(text ?? "Tool returned isError=true");
  }

  if (structuredContent) {
    return structuredContent;
  }

  const text = content?.find((entry) => entry.type === "text")?.text;
  if (!text) {
    return {};
  }

  try {
    return JSON.parse(text) as JsonRecord;
  } catch {
    return { text };
  }
}

function summarizeItems(items: unknown, count = 3): Array<Record<string, unknown>> {
  if (!Array.isArray(items)) {
    return [];
  }

  return items.slice(0, count).map((item) => {
    if (item === null || typeof item !== "object") {
      return { value: item };
    }

    const record = item as JsonRecord;
    const authors = Array.isArray(record.authors)
      ? record.authors
          .slice(0, 2)
          .map((author) =>
            author !== null && typeof author === "object"
              ? (author as JsonRecord).name
              : author,
          )
      : undefined;

    return {
      asin: record.asin,
      title: record.title,
      authors,
    };
  });
}

function ensure(condition: unknown, message: string, failures: string[]): void {
  if (!condition) {
    failures.push(message);
  }
}

function deriveSearchQuery(items: unknown[]): string {
  const firstItem =
    items[0] !== null && typeof items[0] === "object" ? (items[0] as JsonRecord) : undefined;
  const firstAuthor =
    firstItem && Array.isArray(firstItem.authors) && firstItem.authors[0] !== null
      ? (firstItem.authors[0] as JsonRecord | undefined)
      : undefined;

  const authorName =
    firstAuthor && typeof firstAuthor.name === "string" ? firstAuthor.name.trim() : undefined;
  if (authorName) {
    const parts = authorName.split(/\s+/).filter(Boolean);
    const last = parts[parts.length - 1];
    if (last) {
      return last;
    }
  }

  const title = firstItem && typeof firstItem.title === "string" ? firstItem.title.trim() : undefined;
  if (title) {
    const word = title.split(/\s+/).find(Boolean);
    if (word) {
      return word;
    }
  }

  return "audible";
}

async function main(): Promise<void> {
  const authFile =
    parseArg("--auth-file") ??
    process.env.AUDIBLE_AUTH_FILE ??
    path.resolve(process.cwd(), "audible-auth.json");

  const transport = new StdioClientTransport({
    command: "npm",
    args: ["run", "mcp:start"],
    cwd: process.cwd(),
    env: {
      AUDIBLE_AUTH_FILE: authFile,
    },
    stderr: "pipe",
  });

  const client = new Client({
    name: "audible-mcp-smoke-test",
    version: "1.0.0",
  });

  await client.connect(transport);

  try {
    const failures: string[] = [];
    const tools = await client.listTools();
    const toolNames = tools.tools.map((tool) => tool.name);
    for (const name of EXPECTED_TOOLS) {
      ensure(toolNames.includes(name), `Missing tool: ${name}`, failures);
    }

    const authStatus = parseToolResult(
      await client.callTool({ name: "audible_get_auth_status", arguments: {} }),
    );
    const validateAuth = parseToolResult(
      await client.callTool({ name: "audible_validate_auth", arguments: {} }),
    );

    const library = parseToolResult(
      await client.callTool({
        name: "audible_list_library",
        arguments: { numResults: 3, page: 1 },
      }),
    );
    const libraryItems = Array.isArray(library.items) ? library.items : [];
    ensure(libraryItems.length > 0, "Library returned no items.", failures);
    const sampleAsin =
      libraryItems[0] !== null &&
      typeof libraryItems[0] === "object" &&
      typeof (libraryItems[0] as JsonRecord).asin === "string"
        ? ((libraryItems[0] as JsonRecord).asin as string)
        : "B0FVBC49CX";
    const searchQuery = deriveSearchQuery(libraryItems);

    const libraryItem = parseToolResult(
      await client.callTool({
        name: "audible_get_library_item",
        arguments: { asin: sampleAsin },
      }),
    );

    const wishlist = parseToolResult(
      await client.callTool({
        name: "audible_list_wishlist",
        arguments: { numResults: 3, page: 1 },
      }),
    );

    const collections = parseToolResult(
      await client.callTool({ name: "audible_list_collections", arguments: {} }),
    );

    const collectionItems = parseToolResult(
      await client.callTool({
        name: "audible_list_collection_items",
        arguments: { collectionId: "__FAVORITES" },
      }),
    );

    const search = parseToolResult(
      await client.callTool({
        name: "audible_search_library",
        arguments: {
          query: searchQuery,
          maxPages: 2,
          numResultsPerPage: 10,
        },
      }),
    );
    const inProgress = parseToolResult(
      await client.callTool({
        name: "audible_list_in_progress_titles",
        arguments: {
          maxPages: 2,
          numResultsPerPage: 10,
        },
      }),
    );

    const contentMetadata = parseToolResult(
      await client.callTool({
        name: "audible_get_content_metadata",
        arguments: { asin: sampleAsin },
      }),
    );

    const chapters = parseToolResult(
      await client.callTool({
        name: "audible_get_chapters",
        arguments: { asin: sampleAsin },
      }),
    );

    const catalog = parseToolResult(
      await client.callTool({
        name: "audible_get_catalog_product",
        arguments: { asin: sampleAsin },
      }),
    );

    const stats = parseToolResult(
      await client.callTool({
        name: "audible_get_listening_stats",
        arguments: {
          months: 3,
          startMonth: "2026-01",
          store: "Audible",
          locale: "en_US",
        },
      }),
    );

    const resources = await client.listResources();
    const resourceUris = resources.resources.map((resource) => resource.uri);
    for (const uri of EXPECTED_RESOURCES) {
      ensure(resourceUris.includes(uri), `Missing static resource: ${uri}`, failures);
    }
    const resourceTemplates = await client.listResourceTemplates();
    const resourceTemplateUris = resourceTemplates.resourceTemplates.map(
      (resource) => resource.uriTemplate,
    );
    for (const uriTemplate of EXPECTED_RESOURCE_TEMPLATES) {
      ensure(
        resourceTemplateUris.includes(uriTemplate),
        `Missing resource template: ${uriTemplate}`,
        failures,
      );
    }

    const authResource = await client.readResource({ uri: "audible://auth/status" });
    const libraryResource = await client.readResource({ uri: `audible://library/${sampleAsin}` });
    const catalogResource = await client.readResource({ uri: `audible://catalog/${sampleAsin}` });

    ensure(
      typeof authStatus.deviceSerial === "string" && authStatus.deviceSerial.length > 0,
      "Auth status is missing deviceSerial.",
      failures,
    );
    ensure(validateAuth.valid === true, "Auth validation did not return valid=true.", failures);
    const libraryItemRecord =
      libraryItem.item && typeof libraryItem.item === "object"
        ? (libraryItem.item as JsonRecord)
        : libraryItem;
    ensure(
      libraryItemRecord.asin === sampleAsin,
      "Library item response does not match the requested ASIN.",
      failures,
    );
    ensure(
      typeof libraryItemRecord.title === "string" && libraryItemRecord.title.length > 0,
      "Library item response is missing title.",
      failures,
    );
    ensure(
      search.totalMatches !== undefined && Number(search.totalMatches) >= 1,
      `Library search for "${searchQuery}" returned no matches.`,
      failures,
    );
    ensure(
      !!contentMetadata.content_metadata &&
        typeof contentMetadata.content_metadata === "object" &&
        !!(contentMetadata.content_metadata as JsonRecord).chapter_info,
      "Content metadata is missing chapter_info.",
      failures,
    );
    ensure(
      chapters.chapter_info !== null &&
        chapters.chapter_info !== undefined &&
        typeof chapters.chapter_info === "object" &&
        Array.isArray((chapters.chapter_info as JsonRecord).chapters) &&
        ((chapters.chapter_info as JsonRecord).chapters as unknown[]).length > 0,
      "Chapter lookup returned no chapters.",
      failures,
    );
    const product =
      catalog.product && typeof catalog.product === "object"
        ? (catalog.product as JsonRecord)
        : catalog;
    ensure(
      typeof product.title === "string" && product.title.length > 0,
      "Catalog product response is missing title.",
      failures,
    );
    ensure(
      Array.isArray(stats.aggregated_monthly_listening_stats) &&
        stats.aggregated_monthly_listening_stats.length > 0,
      "Listening stats returned no monthly data.",
      failures,
    );
    ensure(
      Array.isArray(inProgress.items),
      "In-progress titles did not return an items array.",
      failures,
    );
    ensure(
      authResource.contents.length > 0,
      "Reading audible://auth/status returned no contents.",
      failures,
    );
    ensure(
      libraryResource.contents.some((entry) => entry.uri === `audible://library/${sampleAsin}`),
      "Reading library resource did not return the requested URI.",
      failures,
    );
    ensure(
      catalogResource.contents.some((entry) => entry.uri === `audible://catalog/${sampleAsin}`),
      "Reading catalog resource did not return the requested URI.",
      failures,
    );

    const summary = {
      checks: {
        failed: failures.length,
        passed: failures.length === 0,
      },
      authStatus: {
        deviceSerial: authStatus.deviceSerial,
        expiresAt: authStatus.expiresAt,
        locale: authStatus.locale,
        marketplaceDomain: authStatus.marketplaceDomain,
      },
      authValidation: validateAuth,
      catalogProduct: (() => {
        const product =
          catalog.product && typeof catalog.product === "object"
            ? (catalog.product as JsonRecord)
            : catalog;
        return {
          asin: product.asin,
          subtitle: product.subtitle,
          title: product.title,
        };
      })(),
      chapters: (() => {
        const chapterInfo =
          chapters.chapter_info && typeof chapters.chapter_info === "object"
            ? (chapters.chapter_info as JsonRecord)
            : {};
        const chapterList = Array.isArray(chapterInfo.chapters) ? chapterInfo.chapters : [];
        return {
          asin: chapters.asin,
          chapterCount: chapterList.length,
          firstThree: chapterList.slice(0, 3).map((chapter) =>
            chapter !== null && typeof chapter === "object"
              ? (chapter as JsonRecord).title
              : chapter,
          ),
        };
      })(),
      collectionItems: {
        collectionId: "__FAVORITES",
        count: Array.isArray(collectionItems.items) ? collectionItems.items.length : 0,
      },
      collections: {
        count: Array.isArray(collections.collections) ? collections.collections.length : 0,
      },
      contentMetadata: {
        chapterCount:
          contentMetadata.content_metadata &&
          typeof contentMetadata.content_metadata === "object" &&
          (contentMetadata.content_metadata as JsonRecord).chapter_info &&
          typeof (contentMetadata.content_metadata as JsonRecord).chapter_info === "object" &&
          Array.isArray(((contentMetadata.content_metadata as JsonRecord).chapter_info as JsonRecord).chapters)
            ? (((contentMetadata.content_metadata as JsonRecord).chapter_info as JsonRecord).chapters as unknown[])
                .length
            : 0,
        hasChapterInfo:
          !!contentMetadata.content_metadata &&
          typeof contentMetadata.content_metadata === "object" &&
          !!(contentMetadata.content_metadata as JsonRecord).chapter_info,
      },
      library: {
        count: libraryItems.length,
        sample: summarizeItems(libraryItems),
      },
      libraryItem: (() => {
        const item =
          libraryItem.item && typeof libraryItem.item === "object"
            ? (libraryItem.item as JsonRecord)
            : libraryItem;
        return {
          asin: item.asin,
          percentComplete: item.percent_complete,
          title: item.title,
        };
      })(),
      inProgressTitles: {
        count: Array.isArray(inProgress.items) ? inProgress.items.length : 0,
        sample: summarizeItems(inProgress.items),
      },
      listeningStats: {
        months: Array.isArray(stats.aggregated_monthly_listening_stats)
          ? stats.aggregated_monthly_listening_stats
          : [],
        totalMs:
          stats.aggregated_total_listening_stats &&
          typeof stats.aggregated_total_listening_stats === "object"
            ? (stats.aggregated_total_listening_stats as JsonRecord).aggregated_sum
            : undefined,
      },
      resources: resources.resources.map((resource) => resource.uri),
      resourceTemplates: resourceTemplateUris,
      searchLibrary: {
        query: search.query,
        sample: summarizeItems(search.items, 3),
        totalMatches: search.totalMatches,
      },
      toolCount: toolNames.length,
      tools: toolNames,
      wishlist: {
        count: Array.isArray(wishlist.products) ? wishlist.products.length : 0,
        sample: summarizeItems(wishlist.products),
      },
    };

    console.log(JSON.stringify(summary, null, 2));

    if (failures.length > 0) {
      console.error("");
      console.error("Smoke test failures:");
      for (const failure of failures) {
        console.error(`- ${failure}`);
      }
      process.exitCode = 1;
    }
  } finally {
    await client.close();
  }
}

main().catch((error) => {
  const message = error instanceof Error ? error.message : String(error);
  console.error(message);
  process.exitCode = 1;
});
