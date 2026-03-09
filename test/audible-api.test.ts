import { generateKeyPairSync } from "node:crypto";
import { mkdtemp, readFile, writeFile } from "node:fs/promises";
import os from "node:os";
import path from "node:path";
import { describe, expect, it, vi } from "vitest";
import type { AudibleAuthFile } from "../src/auth/auth-file.js";
import { SignedAuthStrategy } from "../src/auth/signed-strategy.js";
import { AudibleApi } from "../src/audible/api.js";
import { AudibleClient } from "../src/audible/client.js";

describe("AudibleApi", () => {
  it("requests collection items from the collection endpoint", async () => {
    const fetchFn = vi.fn(async (input: RequestInfo | URL) => {
      const url = input instanceof URL ? input : new URL(String(input));
      return new Response(JSON.stringify({ url: url.toString() }), { status: 200 });
    });

    const api = new AudibleApi(
      new AudibleClient({
        auth: new SignedAuthStrategy(
          {
            adpToken: "adp",
            privateKeyPem: "pem",
          },
          async () => ({ "x-adp-signature": "sig:date", "x-adp-token": "adp" }),
        ),
        baseUrl: "https://api.audible.com",
        fetchFn: fetchFn as typeof fetch,
      }),
    );

    const result = (await api.listCollectionItems({
      collectionId: "__FAVORITES",
    })) as { url: string };

    expect(result.url).toContain("/1.0/collections/__FAVORITES/items");
    expect(result.url).toContain("response_groups=always-returned");
  });

  it("requests a library item with the expected response groups", async () => {
    const fetchFn = vi.fn(async (input: RequestInfo | URL) => {
      const url = input instanceof URL ? input : new URL(String(input));
      return new Response(JSON.stringify({ url: url.toString() }), { status: 200 });
    });

    const api = new AudibleApi(
      new AudibleClient({
        auth: new SignedAuthStrategy(
          {
            adpToken: "adp",
            privateKeyPem: "pem",
          },
          async () => ({ "x-adp-signature": "sig:date", "x-adp-token": "adp" }),
        ),
        baseUrl: "https://api.audible.com",
        fetchFn: fetchFn as typeof fetch,
      }),
    );

    const result = (await api.getLibraryItem("B0FVBC49CX")) as { url: string };

    expect(fetchFn).toHaveBeenCalledOnce();
    expect(result.url).toContain("/1.0/library/B0FVBC49CX");
    expect(result.url).toContain("response_groups=contributors%2Cmedia%2Cproduct_attrs");
  });

  it("requests listening stats with the required store parameter", async () => {
    const fetchFn = vi.fn(async (input: RequestInfo | URL) => {
      const url = input instanceof URL ? input : new URL(String(input));
      return new Response(JSON.stringify({ search: url.search }), { status: 200 });
    });

    const api = new AudibleApi(
      new AudibleClient({
        auth: new SignedAuthStrategy(
          {
            adpToken: "adp",
            privateKeyPem: "pem",
          },
          async () => ({ "x-adp-signature": "sig:date", "x-adp-token": "adp" }),
        ),
        baseUrl: "https://api.audible.com",
        fetchFn: fetchFn as typeof fetch,
      }),
    );

    const result = (await api.getListeningStats({
      months: 2,
      startMonth: "2026-02",
    })) as { search: string };

    expect(result.search).toContain("monthly_listening_interval_duration=2");
    expect(result.search).toContain("monthly_listening_interval_start_date=2026-02");
    expect(result.search).toContain("store=Audible");
  });

  it("extracts chapter_info from content metadata", async () => {
    const fetchFn = vi.fn(async () => {
      return new Response(
        JSON.stringify({
          content_metadata: {
            chapter_info: {
              chapters: [{ title: "Chapter 1" }],
            },
          },
        }),
        { status: 200 },
      );
    });

    const api = new AudibleApi(
      new AudibleClient({
        auth: new SignedAuthStrategy(
          {
            adpToken: "adp",
            privateKeyPem: "pem",
          },
          async () => ({ "x-adp-signature": "sig:date", "x-adp-token": "adp" }),
        ),
        baseUrl: "https://api.audible.com",
        fetchFn: fetchFn as typeof fetch,
      }),
    );

    const result = (await api.getChapters("B0FVBC49CX")) as {
      asin: string;
      chapter_info: { chapters: Array<{ title: string }> };
    };

    expect(result.asin).toBe("B0FVBC49CX");
    expect(result.chapter_info.chapters[0]?.title).toBe("Chapter 1");
  });

  it("searches library items across pages using client-side matching", async () => {
    const fetchFn = vi.fn(async (input: RequestInfo | URL) => {
      const url = input instanceof URL ? input : new URL(String(input));
      const page = url.searchParams.get("page");

      if (page === "1") {
        return new Response(
          JSON.stringify({
            items: [
              {
                asin: "A1",
                title: "Nothing Relevant",
                authors: [{ name: "Someone Else" }],
              },
            ],
          }),
          { status: 200 },
        );
      }

      return new Response(
        JSON.stringify({
          items: [
            {
              asin: "A2",
              title: "The Mathematician's Mind",
              authors: [{ name: "Rebecca Goldstein" }],
            },
          ],
        }),
        { status: 200 },
      );
    });

    const api = new AudibleApi(
      new AudibleClient({
        auth: new SignedAuthStrategy(
          {
            adpToken: "adp",
            privateKeyPem: "pem",
          },
          async () => ({ "x-adp-signature": "sig:date", "x-adp-token": "adp" }),
        ),
        baseUrl: "https://api.audible.com",
        fetchFn: fetchFn as typeof fetch,
      }),
    );

    const result = (await api.searchLibrary({
      maxPages: 2,
      numResultsPerPage: 1,
      query: "goldstein",
    })) as { items: Array<{ asin: string }>; totalMatches: number };

    expect(fetchFn).toHaveBeenCalledTimes(2);
    expect(result.totalMatches).toBe(1);
    expect(result.items[0]?.asin).toBe("A2");
  });

  it("lists in-progress titles using percent_complete and is_finished fields", async () => {
    const fetchFn = vi.fn(async () => {
      return new Response(
        JSON.stringify({
          items: [
            {
              asin: "A1",
              is_finished: false,
              percent_complete: 42.5,
              title: "In Progress",
            },
            {
              asin: "A2",
              is_finished: true,
              percent_complete: 100,
              title: "Finished",
            },
          ],
        }),
        { status: 200 },
      );
    });

    const api = new AudibleApi(
      new AudibleClient({
        auth: new SignedAuthStrategy(
          {
            adpToken: "adp",
            privateKeyPem: "pem",
          },
          async () => ({ "x-adp-signature": "sig:date", "x-adp-token": "adp" }),
        ),
        baseUrl: "https://api.audible.com",
        fetchFn: fetchFn as typeof fetch,
      }),
    );

    const result = (await api.listInProgressTitles({
      maxPages: 1,
      numResultsPerPage: 25,
    })) as { items: Array<{ asin: string }>; totalMatches: number };

    expect(result.totalMatches).toBe(1);
    expect(result.items[0]?.asin).toBe("A1");
  });

  it("refreshes expired auth files before making signed requests", async () => {
    const tempDir = await mkdtemp(path.join(os.tmpdir(), "audible-mcp-"));
    const authFilePath = path.join(tempDir, "audible-auth.json");
    const { privateKey } = generateKeyPairSync("rsa", {
      modulusLength: 2048,
      privateKeyEncoding: { format: "pem", type: "pkcs1" },
      publicKeyEncoding: { format: "pem", type: "pkcs1" },
    });

    const authFile: AudibleAuthFile = {
      accessToken: "expired-access",
      adpToken: "adp-token",
      devicePrivateKey: privateKey,
      domain: "com",
      expiresAt: Date.now() - 1000,
      locale: "us",
      refreshToken: "refresh-token",
      serial: "SERIAL123",
      withUsername: false,
    };
    await writeFile(authFilePath, JSON.stringify(authFile, null, 2), "utf8");

    const originalFetch = globalThis.fetch;
    const fetchMock = vi.fn(async (input: RequestInfo | URL, init?: RequestInit) => {
      const url = input instanceof URL ? input.toString() : String(input);
      if (url.includes("/auth/token")) {
        const body = init?.body;
        expect(body instanceof URLSearchParams).toBe(true);
        expect(body?.toString()).toContain("source_token=refresh-token");
        return new Response(
          JSON.stringify({
            access_token: "fresh-access",
            expires_in: 3600,
          }),
          { status: 200 },
        );
      }

      expect(new Headers(init?.headers).get("x-adp-token")).toBe("adp-token");
      return new Response(JSON.stringify({ items: [{ asin: "A1", title: "Fresh Title" }] }), {
        status: 200,
      });
    });
    globalThis.fetch = fetchMock as typeof fetch;

    try {
      const api = await AudibleApi.fromAuthFile({ authFile: authFilePath });
      const result = (await api.validateAuth()) as { sampleLibraryCount: number; valid: boolean };
      const saved = JSON.parse(await readFile(authFilePath, "utf8")) as AudibleAuthFile;

      expect(result.valid).toBe(true);
      expect(result.sampleLibraryCount).toBe(1);
      expect(saved.accessToken).toBe("fresh-access");
      expect(fetchMock).toHaveBeenCalledTimes(2);
    } finally {
      globalThis.fetch = originalFetch;
    }
  });
});
