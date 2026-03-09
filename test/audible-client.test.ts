import { describe, expect, it, vi } from "vitest";
import { SignedAuthStrategy } from "../src/auth/signed-strategy.js";
import type { AuthStrategy } from "../src/auth/types.js";
import { AudibleClient } from "../src/audible/client.js";

describe("AudibleClient", () => {
  it("applies auth headers and query params", async () => {
    const fetchFn = vi.fn(async (input: RequestInfo | URL, init?: RequestInit) => {
      const url = input instanceof URL ? input : new URL(String(input));
      return new Response(
        JSON.stringify({
          custom: new Headers(init?.headers).get("x-test-auth"),
          search: url.search,
        }),
        { status: 200 },
      );
    });

    const auth: AuthStrategy = {
      kind: "signed",
      name: "test",
      async apply(context) {
        context.headers.set("x-test-auth", "ok");
      },
    };

    const client = new AudibleClient({
      auth,
      baseUrl: "https://api.audible.com",
      fetchFn: fetchFn as typeof fetch,
    });

    const response = await client.request("/1.0/library", {
      query: {
        page: 2,
      },
    });

    expect(fetchFn).toHaveBeenCalledOnce();
    expect(response.status).toBe(200);
    expect(response.bodyText).toContain("\"custom\":\"ok\"");
    expect(response.bodyText).toContain("?page=2");
  });

  it("merges signed headers from the signer module contract", async () => {
    const fetchFn = vi.fn(async (_input: RequestInfo | URL, init?: RequestInit) => {
      return new Response(
        JSON.stringify({
          adp: new Headers(init?.headers).get("x-adp-token"),
          signature: new Headers(init?.headers).get("x-signature"),
        }),
        { status: 200 },
      );
    });

    const client = new AudibleClient({
      auth: new SignedAuthStrategy(
        {
          adpToken: "adp-456",
          privateKeyPem: "pem",
        },
        async () => ({ "x-signature": "signed" }),
      ),
      baseUrl: "https://api.audible.com",
      fetchFn: fetchFn as typeof fetch,
    });

    const response = await client.request("/1.0/account");

    expect(response.bodyText).toContain("\"adp\":\"adp-456\"");
    expect(response.bodyText).toContain("\"signature\":\"signed\"");
  });
});
