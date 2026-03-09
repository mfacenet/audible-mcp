import { refreshAudibleAccessToken, signAudibleRequest } from "../auth/audible-auth.js";
import type { AudibleAuthFile } from "../auth/auth-file.js";
import { loadAuthFile, saveAuthFile } from "../auth/auth-file.js";
import { SignedAuthStrategy } from "../auth/signed-strategy.js";
import { AudibleClient } from "./client.js";

export interface AudibleApiOptions {
  authFile: string;
  baseUrl?: string;
}

export interface LibraryListOptions {
  numResults?: number;
  page?: number;
}

export interface WishlistListOptions {
  numResults?: number;
  page?: number;
}

export interface CollectionItemsOptions {
  collectionId: string;
}

export interface SearchLibraryOptions {
  maxPages?: number;
  numResultsPerPage?: number;
  query: string;
}

export interface InProgressTitlesOptions {
  maxPages?: number;
  numResultsPerPage?: number;
}

export interface ListeningStatsOptions {
  locale?: string;
  months?: number;
  startMonth?: string;
  store?: string;
}

const DEFAULT_BASE_URL = "https://api.audible.com";
const DEFAULT_LIBRARY_RESPONSE_GROUPS = [
  "contributors",
  "media",
  "product_attrs",
  "product_desc",
  "rating",
  "series",
];
const DEFAULT_LIBRARY_ITEM_RESPONSE_GROUPS = [
  ...DEFAULT_LIBRARY_RESPONSE_GROUPS,
  "is_downloaded",
  "is_finished",
  "pdf_url",
  "percent_complete",
];
const DEFAULT_CONTENT_METADATA_RESPONSE_GROUPS = ["chapter_info", "content_reference"];
const DEFAULT_CATALOG_RESPONSE_GROUPS = [...DEFAULT_LIBRARY_RESPONSE_GROUPS, "sku"];
const AUTH_REFRESH_WINDOW_MS = 5 * 60 * 1000;

function parseJsonBody(bodyText: string): unknown {
  try {
    return JSON.parse(bodyText) as unknown;
  } catch {
    return bodyText;
  }
}

function createApiErrorMessage(status: number, body: unknown): string {
  const payload =
    typeof body === "string" ? body : JSON.stringify(body, null, 2).slice(0, 1200);
  return `Audible API request failed with status ${status}: ${payload}`;
}

function monthStringFromDate(date: Date): string {
  const year = date.getUTCFullYear();
  const month = `${date.getUTCMonth() + 1}`.padStart(2, "0");
  return `${year}-${month}`;
}

function defaultStartMonth(): string {
  const date = new Date();
  date.setUTCMonth(date.getUTCMonth() - 2);
  return monthStringFromDate(date);
}

function normalizeCount(value: number | undefined, fallback: number, max: number): number {
  if (value === undefined) {
    return fallback;
  }
  return Math.max(1, Math.min(max, Math.trunc(value)));
}

function normalizeText(value: string): string {
  return value.trim().toLocaleLowerCase();
}

function valueMatchesQuery(value: unknown, normalizedQuery: string): boolean {
  return typeof value === "string" && normalizeText(value).includes(normalizedQuery);
}

function itemMatchesLibraryQuery(item: Record<string, unknown>, normalizedQuery: string): boolean {
  if (valueMatchesQuery(item.asin, normalizedQuery) || valueMatchesQuery(item.title, normalizedQuery)) {
    return true;
  }

  const creators = ["authors", "narrators"];
  for (const key of creators) {
    const values = item[key];
    if (!Array.isArray(values)) {
      continue;
    }

    for (const entry of values) {
      if (
        entry !== null &&
        typeof entry === "object" &&
        valueMatchesQuery((entry as Record<string, unknown>).name, normalizedQuery)
      ) {
        return true;
      }
    }
  }

  const series = item.series;
  if (Array.isArray(series)) {
    for (const entry of series) {
      if (
        entry !== null &&
        typeof entry === "object" &&
        valueMatchesQuery((entry as Record<string, unknown>).name, normalizedQuery)
      ) {
        return true;
      }
    }
  }

  return false;
}

export class AudibleApi {
  private client: AudibleClient;

  constructor(
    client: AudibleClient,
    private readonly authContext?: {
      authFile: AudibleAuthFile;
      authFilePath: string;
      baseUrl: string;
    },
  ) {
    this.client = client;
  }

  private static createSignedClient(authFile: AudibleAuthFile, baseUrl: string): AudibleClient {
    return new AudibleClient({
      auth: new SignedAuthStrategy(
        {
          adpToken: authFile.adpToken,
          privateKeyPem: authFile.devicePrivateKey,
        },
        signAudibleRequest,
      ),
      baseUrl,
    });
  }

  static async fromAuthFile(options: AudibleApiOptions): Promise<AudibleApi> {
    const authFile = await loadAuthFile(options.authFile);
    const baseUrl = options.baseUrl ?? DEFAULT_BASE_URL;
    const client = AudibleApi.createSignedClient(authFile, baseUrl);

    return new AudibleApi(client, {
      authFile,
      authFilePath: options.authFile,
      baseUrl,
    });
  }

  private async refreshAccessToken(force = false): Promise<void> {
    if (!this.authContext) {
      return;
    }

    if (!force && this.authContext.authFile.expiresAt > Date.now() + AUTH_REFRESH_WINDOW_MS) {
      return;
    }

    const refreshed = await refreshAudibleAccessToken(this.authContext.authFile);
    this.authContext.authFile.accessToken = refreshed.accessToken;
    this.authContext.authFile.expiresAt = refreshed.expiresAt;
    await saveAuthFile(this.authContext.authFilePath, this.authContext.authFile);
    this.client = AudibleApi.createSignedClient(this.authContext.authFile, this.authContext.baseUrl);
  }

  private async getJson<T>(
    pathname: string,
    query?: Record<string, string | number | boolean | undefined>,
  ): Promise<T> {
    await this.refreshAccessToken();
    let response = await this.client.request(
      pathname,
      query === undefined ? { method: "GET" } : { method: "GET", query },
    );
    let body = parseJsonBody(response.bodyText);

    if ((response.status === 401 || response.status === 403) && this.authContext) {
      await this.refreshAccessToken(true);
      response = await this.client.request(
        pathname,
        query === undefined ? { method: "GET" } : { method: "GET", query },
      );
      body = parseJsonBody(response.bodyText);
    }

    if (!response.ok) {
      throw new Error(createApiErrorMessage(response.status, body));
    }

    return body as T;
  }

  async listLibrary(options: LibraryListOptions = {}): Promise<unknown> {
    return this.getLibraryPage({
      num_results: normalizeCount(options.numResults, 25, 100),
      page: normalizeCount(options.page, 1, 1000),
      response_groups: DEFAULT_LIBRARY_RESPONSE_GROUPS.join(","),
    });
  }

  async getLibraryItem(asin: string): Promise<unknown> {
    return this.getJson(`/1.0/library/${asin}`, {
      response_groups: DEFAULT_LIBRARY_ITEM_RESPONSE_GROUPS.join(","),
    });
  }

  async listWishlist(options: WishlistListOptions = {}): Promise<unknown> {
    return this.getJson("/1.0/wishlist", {
      num_results: normalizeCount(options.numResults, 25, 100),
      page: normalizeCount(options.page, 1, 1000),
      response_groups: DEFAULT_LIBRARY_RESPONSE_GROUPS.join(","),
    });
  }

  async listCollections(): Promise<unknown> {
    return this.getJson("/1.0/collections");
  }

  private async getLibraryPage(
    query: Record<string, string | number | boolean | undefined>,
  ): Promise<unknown> {
    return this.getJson("/1.0/library", query);
  }

  async listCollectionItems(options: CollectionItemsOptions): Promise<unknown> {
    return this.getJson(`/1.0/collections/${options.collectionId}/items`, {
      response_groups: "always-returned",
    });
  }

  async searchLibrary(options: SearchLibraryOptions): Promise<unknown> {
    const normalizedQuery = normalizeText(options.query);
    const maxPages = normalizeCount(options.maxPages, 3, 20);
    const numResultsPerPage = normalizeCount(options.numResultsPerPage, 25, 100);
    const matches: Record<string, unknown>[] = [];
    const seenAsins = new Set<string>();

    for (let page = 1; page <= maxPages; page += 1) {
      const response = (await this.listLibrary({
        numResults: numResultsPerPage,
        page,
      })) as { items?: unknown[]; total_count?: number };

      const items = Array.isArray(response.items) ? response.items : [];
      for (const item of items) {
        if (item === null || typeof item !== "object") {
          continue;
        }

        const record = item as Record<string, unknown>;
        if (!itemMatchesLibraryQuery(record, normalizedQuery)) {
          continue;
        }

        const asin = typeof record.asin === "string" ? record.asin : undefined;
        if (asin && seenAsins.has(asin)) {
          continue;
        }
        if (asin) {
          seenAsins.add(asin);
        }
        matches.push(record);
      }

      if (items.length < numResultsPerPage) {
        break;
      }
    }

    return {
      items: matches,
      query: options.query,
      searchedPages: maxPages,
      totalMatches: matches.length,
    };
  }

  async listInProgressTitles(options: InProgressTitlesOptions = {}): Promise<unknown> {
    const maxPages = normalizeCount(options.maxPages, 3, 20);
    const numResultsPerPage = normalizeCount(options.numResultsPerPage, 25, 100);
    const items: Record<string, unknown>[] = [];

    for (let page = 1; page <= maxPages; page += 1) {
      const response = (await this.getLibraryPage({
        num_results: numResultsPerPage,
        page,
        response_groups: DEFAULT_LIBRARY_ITEM_RESPONSE_GROUPS.join(","),
      })) as { items?: unknown[] };

      const pageItems = Array.isArray(response.items) ? response.items : [];
      for (const item of pageItems) {
        if (item === null || typeof item !== "object") {
          continue;
        }

        const record = item as Record<string, unknown>;
        const percentComplete =
          typeof record.percent_complete === "number" ? record.percent_complete : undefined;
        const isFinished =
          typeof record.is_finished === "boolean" ? record.is_finished : undefined;
        const isInProgress =
          (percentComplete !== undefined && percentComplete > 0 && percentComplete < 100) ||
          isFinished === false;

        if (isInProgress) {
          items.push(record);
        }
      }

      if (pageItems.length < numResultsPerPage) {
        break;
      }
    }

    return {
      items,
      scannedPages: maxPages,
      totalMatches: items.length,
    };
  }

  async getContentMetadata(asin: string): Promise<unknown> {
    return this.getJson(`/1.0/content/${asin}/metadata`, {
      response_groups: DEFAULT_CONTENT_METADATA_RESPONSE_GROUPS.join(","),
    });
  }

  async getChapters(asin: string): Promise<unknown> {
    const metadata = (await this.getContentMetadata(asin)) as {
      content_metadata?: {
        chapter_info?: unknown;
      };
    };

    return {
      asin,
      chapter_info: metadata.content_metadata?.chapter_info ?? null,
    };
  }

  async getCatalogProduct(asin: string): Promise<unknown> {
    return this.getJson(`/1.0/catalog/products/${asin}`, {
      response_groups: DEFAULT_CATALOG_RESPONSE_GROUPS.join(","),
    });
  }

  async getListeningStats(options: ListeningStatsOptions = {}): Promise<unknown> {
    return this.getJson("/1.0/stats/aggregates", {
      locale: options.locale ?? "en_US",
      monthly_listening_interval_duration: normalizeCount(options.months, 3, 24),
      monthly_listening_interval_start_date: options.startMonth ?? defaultStartMonth(),
      response_groups: "total_listening_stats",
      store: options.store ?? "Audible",
    });
  }

  async validateAuth(): Promise<unknown> {
    await this.refreshAccessToken();
    const library = (await this.listLibrary({ numResults: 1, page: 1 })) as {
      items?: unknown[];
    };

    return {
      expiresAt:
        this.authContext !== undefined
          ? new Date(this.authContext.authFile.expiresAt).toISOString()
          : null,
      sampleLibraryCount: Array.isArray(library.items) ? library.items.length : 0,
      valid: true,
    };
  }
}
