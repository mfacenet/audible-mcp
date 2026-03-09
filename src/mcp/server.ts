import { McpServer, ResourceTemplate } from "@modelcontextprotocol/sdk/server/mcp.js";
import type { CallToolResult } from "@modelcontextprotocol/sdk/types.js";
import * as z from "zod/v4";
import { loadAuthFile } from "../auth/auth-file.js";
import { AudibleApi } from "../audible/api.js";

export interface CreateAudibleMcpServerOptions {
  authFile: string;
  baseUrl?: string;
}

function jsonText(value: unknown): string {
  return JSON.stringify(value, null, 2);
}

function createJsonToolResult(value: unknown): CallToolResult {
  return {
    content: [
      {
        type: "text",
        text: jsonText(value),
      },
    ],
    structuredContent:
      value !== null && typeof value === "object"
        ? (value as Record<string, unknown>)
        : { value },
  };
}

function createErrorToolResult(error: unknown): CallToolResult {
  const message = error instanceof Error ? error.message : String(error);
  return {
    content: [
      {
        type: "text",
        text: message,
      },
    ],
    isError: true,
  };
}

async function createAudibleApi(options: CreateAudibleMcpServerOptions): Promise<AudibleApi> {
  return AudibleApi.fromAuthFile(options);
}

export async function createAudibleMcpServer(
  options: CreateAudibleMcpServerOptions,
): Promise<McpServer> {
  const authFile = await loadAuthFile(options.authFile);
  const api = await createAudibleApi(options);

  const server = new McpServer(
    {
      name: "audible-mcp",
      version: "1.0.0",
    },
    {
      instructions:
        "Use the Audible tools to inspect the user's library, wishlist, collections, catalog metadata, and listening stats. Prefer read-only workflows.",
    },
  );

  server.registerTool(
    "audible_list_library",
    {
      description: "List titles from the authenticated Audible library.",
      inputSchema: {
        page: z.number().int().positive().optional().describe("1-based results page."),
        numResults: z
          .number()
          .int()
          .positive()
          .max(100)
          .optional()
          .describe("Number of items to return, up to 100."),
      },
    },
    async ({ numResults, page }) => {
      try {
        return createJsonToolResult(
          await api.listLibrary({
            ...(numResults !== undefined ? { numResults } : {}),
            ...(page !== undefined ? { page } : {}),
          }),
        );
      } catch (error) {
        return createErrorToolResult(error);
      }
    },
  );

  server.registerTool(
    "audible_get_library_item",
    {
      description: "Fetch a single library item by ASIN, including progress and download-related fields.",
      inputSchema: {
        asin: z.string().min(1).describe("Audible ASIN."),
      },
    },
    async ({ asin }) => {
      try {
        return createJsonToolResult(await api.getLibraryItem(asin));
      } catch (error) {
        return createErrorToolResult(error);
      }
    },
  );

  server.registerTool(
    "audible_list_wishlist",
    {
      description: "List titles in the authenticated Audible wishlist.",
      inputSchema: {
        page: z.number().int().positive().optional().describe("1-based results page."),
        numResults: z
          .number()
          .int()
          .positive()
          .max(100)
          .optional()
          .describe("Number of items to return, up to 100."),
      },
    },
    async ({ numResults, page }) => {
      try {
        return createJsonToolResult(
          await api.listWishlist({
            ...(numResults !== undefined ? { numResults } : {}),
            ...(page !== undefined ? { page } : {}),
          }),
        );
      } catch (error) {
        return createErrorToolResult(error);
      }
    },
  );

  server.registerTool(
    "audible_list_collections",
    {
      description: "List Audible collections for the authenticated account.",
      inputSchema: {},
    },
    async () => {
      try {
        return createJsonToolResult(await api.listCollections());
      } catch (error) {
        return createErrorToolResult(error);
      }
    },
  );

  server.registerTool(
    "audible_list_collection_items",
    {
      description: "List items inside an Audible collection by collection id.",
      inputSchema: {
        collectionId: z.string().min(1).describe("Collection id, such as __FAVORITES."),
      },
    },
    async ({ collectionId }) => {
      try {
        return createJsonToolResult(await api.listCollectionItems({ collectionId }));
      } catch (error) {
        return createErrorToolResult(error);
      }
    },
  );

  server.registerTool(
    "audible_search_library",
    {
      description:
        "Search the authenticated library by title, ASIN, author, narrator, or series name using client-side filtering across the first few pages.",
      inputSchema: {
        query: z.string().min(1).describe("Text to match against titles and contributor names."),
        maxPages: z
          .number()
          .int()
          .positive()
          .max(20)
          .optional()
          .describe("How many library pages to scan."),
        numResultsPerPage: z
          .number()
          .int()
          .positive()
          .max(100)
          .optional()
          .describe("Page size to fetch from the library endpoint."),
      },
    },
    async ({ maxPages, numResultsPerPage, query }) => {
      try {
        return createJsonToolResult(
          await api.searchLibrary({
            query,
            ...(maxPages !== undefined ? { maxPages } : {}),
            ...(numResultsPerPage !== undefined ? { numResultsPerPage } : {}),
          }),
        );
      } catch (error) {
        return createErrorToolResult(error);
      }
    },
  );

  server.registerTool(
    "audible_list_in_progress_titles",
    {
      description:
        "List titles in the library that have started but are not yet finished.",
      inputSchema: {
        maxPages: z
          .number()
          .int()
          .positive()
          .max(20)
          .optional()
          .describe("How many library pages to scan."),
        numResultsPerPage: z
          .number()
          .int()
          .positive()
          .max(100)
          .optional()
          .describe("Page size to fetch from the library endpoint."),
      },
    },
    async ({ maxPages, numResultsPerPage }) => {
      try {
        return createJsonToolResult(
          await api.listInProgressTitles({
            ...(maxPages !== undefined ? { maxPages } : {}),
            ...(numResultsPerPage !== undefined ? { numResultsPerPage } : {}),
          }),
        );
      } catch (error) {
        return createErrorToolResult(error);
      }
    },
  );

  server.registerTool(
    "audible_get_content_metadata",
    {
      description: "Fetch content metadata for an Audible ASIN, including chapter information.",
      inputSchema: {
        asin: z.string().min(1).describe("Audible ASIN."),
      },
    },
    async ({ asin }) => {
      try {
        return createJsonToolResult(await api.getContentMetadata(asin));
      } catch (error) {
        return createErrorToolResult(error);
      }
    },
  );

  server.registerTool(
    "audible_get_chapters",
    {
      description: "Fetch chapter information for an Audible ASIN.",
      inputSchema: {
        asin: z.string().min(1).describe("Audible ASIN."),
      },
    },
    async ({ asin }) => {
      try {
        return createJsonToolResult(await api.getChapters(asin));
      } catch (error) {
        return createErrorToolResult(error);
      }
    },
  );

  server.registerTool(
    "audible_get_catalog_product",
    {
      description: "Fetch catalog metadata for an Audible ASIN from the catalog products endpoint.",
      inputSchema: {
        asin: z.string().min(1).describe("Audible ASIN."),
      },
    },
    async ({ asin }) => {
      try {
        return createJsonToolResult(await api.getCatalogProduct(asin));
      } catch (error) {
        return createErrorToolResult(error);
      }
    },
  );

  server.registerTool(
    "audible_get_listening_stats",
    {
      description: "Fetch aggregate listening stats. The default window is the current month plus the prior two months.",
      inputSchema: {
        startMonth: z
          .string()
          .regex(/^\d{4}-\d{2}$/)
          .optional()
          .describe("Start month in YYYY-MM format."),
        months: z
          .number()
          .int()
          .positive()
          .max(24)
          .optional()
          .describe("Number of monthly buckets to fetch."),
        locale: z.string().optional().describe("Audible locale, for example en_US."),
        store: z.string().optional().describe("Stats store. Audible works for standard audiobook stats."),
      },
    },
    async ({ locale, months, startMonth, store }) => {
      try {
        return createJsonToolResult(
          await api.getListeningStats({
            ...(locale !== undefined ? { locale } : {}),
            ...(months !== undefined ? { months } : {}),
            ...(startMonth !== undefined ? { startMonth } : {}),
            ...(store !== undefined ? { store } : {}),
          }),
        );
      } catch (error) {
        return createErrorToolResult(error);
      }
    },
  );

  server.registerTool(
    "audible_get_auth_status",
    {
      description: "Return local auth metadata for the configured Audible device registration.",
      inputSchema: {},
    },
    async () =>
      createJsonToolResult({
        authFile: options.authFile,
        deviceSerial: authFile.serial,
        expiresAt: new Date(authFile.expiresAt).toISOString(),
        locale: authFile.locale,
        marketplaceDomain: authFile.domain,
        websiteCookieNames: Object.keys(authFile.websiteCookies ?? {}),
      }),
  );

  server.registerTool(
    "audible_validate_auth",
    {
      description:
        "Validate signed-auth access by performing a lightweight library read and refreshing tokens if needed.",
      inputSchema: {},
    },
    async () => {
      try {
        return createJsonToolResult(await api.validateAuth());
      } catch (error) {
        return createErrorToolResult(error);
      }
    },
  );

  server.registerResource(
    "audible-auth-status",
    "audible://auth/status",
    {
      description: "Current signed-auth device metadata for the configured Audible auth file.",
      mimeType: "application/json",
    },
    async () => ({
      contents: [
        {
          mimeType: "application/json",
          text: jsonText({
            deviceSerial: authFile.serial,
            expiresAt: new Date(authFile.expiresAt).toISOString(),
            locale: authFile.locale,
            marketplaceDomain: authFile.domain,
          }),
          uri: "audible://auth/status",
        },
      ],
    }),
  );

  server.registerResource(
    "audible-wishlist",
    "audible://wishlist",
    {
      description: "Current Audible wishlist.",
      mimeType: "application/json",
    },
    async () => ({
      contents: [
        {
          mimeType: "application/json",
          text: jsonText(await api.listWishlist()),
          uri: "audible://wishlist",
        },
      ],
    }),
  );

  server.registerResource(
    "audible-collections",
    "audible://collections",
    {
      description: "Current Audible collections list.",
      mimeType: "application/json",
    },
    async () => ({
      contents: [
        {
          mimeType: "application/json",
          text: jsonText(await api.listCollections()),
          uri: "audible://collections",
        },
      ],
    }),
  );

  server.registerResource(
    "audible-library-item",
    new ResourceTemplate("audible://library/{asin}", {
      list: undefined,
    }),
    {
      description: "Read a library item by ASIN.",
      mimeType: "application/json",
    },
    async (_uri, params) => ({
      contents: [
        {
          mimeType: "application/json",
          text: jsonText(await api.getLibraryItem(String(params.asin))),
          uri: `audible://library/${params.asin}`,
        },
      ],
    }),
  );

  server.registerResource(
    "audible-collection-items",
    new ResourceTemplate("audible://collections/{collectionId}/items", {
      list: undefined,
    }),
    {
      description: "Read items from a collection by collection id.",
      mimeType: "application/json",
    },
    async (_uri, params) => ({
      contents: [
        {
          mimeType: "application/json",
          text: jsonText(await api.listCollectionItems({ collectionId: String(params.collectionId) })),
          uri: `audible://collections/${params.collectionId}/items`,
        },
      ],
    }),
  );

  server.registerResource(
    "audible-content-metadata",
    new ResourceTemplate("audible://content/{asin}/metadata", {
      list: undefined,
    }),
    {
      description: "Read chapter and content metadata by ASIN.",
      mimeType: "application/json",
    },
    async (_uri, params) => ({
      contents: [
        {
          mimeType: "application/json",
          text: jsonText(await api.getContentMetadata(String(params.asin))),
          uri: `audible://content/${params.asin}/metadata`,
        },
      ],
    }),
  );

  server.registerResource(
    "audible-catalog-product",
    new ResourceTemplate("audible://catalog/{asin}", {
      list: undefined,
    }),
    {
      description: "Read catalog product metadata by ASIN.",
      mimeType: "application/json",
    },
    async (_uri, params) => ({
      contents: [
        {
          mimeType: "application/json",
          text: jsonText(await api.getCatalogProduct(String(params.asin))),
          uri: `audible://catalog/${params.asin}`,
        },
      ],
    }),
  );

  return server;
}
