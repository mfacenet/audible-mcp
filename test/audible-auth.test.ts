import { generateKeyPairSync, verify } from "node:crypto";
import { describe, expect, it } from "vitest";
import {
  buildClientId,
  buildDeviceSerial,
  createAudibleAuthFile,
  createExternalLoginSession,
  extractAuthorizationCode,
  refreshAudibleAccessToken,
  refreshAudibleWebsiteCookies,
  registerAudibleDevice,
  signAudibleRequest,
} from "../src/auth/audible-auth.js";
import { getMarketplace } from "../src/audible/marketplaces.js";

describe("audible auth helpers", () => {
  it("builds Audible iOS client ids from the device serial", () => {
    expect(buildClientId("ABC123")).toBe(Buffer.from("ABC123#A2CZJZGLK2JJVM").toString("hex"));
  });

  it("creates uppercase serials", () => {
    expect(buildDeviceSerial()).toMatch(/^[A-F0-9]{32}$/);
  });

  it("creates login sessions that include the marketplace and PKCE parameters", () => {
    const session = createExternalLoginSession(getMarketplace("us"), {
      serial: "ABC123",
    });

    expect(session.serial).toBe("ABC123");
    expect(session.loginUrl).toContain("openid.oa2.code_challenge_method=S256");
    expect(session.loginUrl).toContain("marketPlaceId=AF2M0KC94RCEA");
    expect(session.loginUrl).toContain(`device%3A${buildClientId("ABC123")}`);
  });

  it("extracts the authorization code from the maplanding URL", () => {
    const code = extractAuthorizationCode(
      "https://www.amazon.com/ap/maplanding?openid.oa2.authorization_code=abc123",
    );
    expect(code).toBe("abc123");
  });

  it("normalizes registration responses into an auth file shape", async () => {
    const session = createExternalLoginSession(getMarketplace("us"), {
      serial: "ABC123",
    });

    const registration = await registerAudibleDevice(session, "code-123", async () =>
      new Response(
        JSON.stringify({
          response: {
            success: {
              extensions: {
                customer_info: { id: "customer" },
                device_info: { serial: "ABC123" },
              },
              tokens: {
                bearer: {
                  access_token: "access",
                  expires_in: 3600,
                  refresh_token: "refresh",
                },
                mac_dms: {
                  adp_token: "adp",
                  device_private_key: "pem",
                },
                website_cookies: [{ Name: "at-main", Value: '"cookie"' }],
              },
            },
          },
        }),
        { status: 200 },
      ),
    );

    const authFile = createAudibleAuthFile(session, registration);
    expect(authFile.accessToken).toBe("access");
    expect(authFile.websiteCookies?.["at-main"]).toBe("cookie");
    expect(authFile.serial).toBe("ABC123");
  });

  it("refreshes access tokens with the documented request shape", async () => {
    const next = await refreshAudibleAccessToken(
      {
        domain: "com",
        refreshToken: "refresh",
        withUsername: false,
      },
      async (_input, init) => {
        const body = init?.body;
        expect(body instanceof URLSearchParams).toBe(true);
        expect(body?.toString()).toContain("requested_token_type=access_token");
        return new Response(JSON.stringify({ access_token: "new-access", expires_in: 1200 }), {
          status: 200,
        });
      },
    );

    expect(next.accessToken).toBe("new-access");
    expect(next.expiresAt).toBeGreaterThan(Date.now());
  });

  it("refreshes website cookies using the exchange endpoint", async () => {
    const cookies = await refreshAudibleWebsiteCookies(
      {
        domain: "com",
        refreshToken: "refresh",
        withUsername: false,
      },
      "com",
      async (_input, init) => {
        const body = init?.body;
        expect(body instanceof URLSearchParams).toBe(true);
        expect(body?.toString()).toContain("requested_token_type=auth_cookies");
        return new Response(
          JSON.stringify({
            response: {
              tokens: {
                cookies: {
                  ".amazon.com": [{ Name: "sess-at-main", Value: '"cookie-value"' }],
                },
              },
            },
          }),
          { status: 200 },
        );
      },
    );

    expect(cookies["sess-at-main"]).toBe("cookie-value");
  });

  it("signs requests with RSA-SHA256 in the documented header format", async () => {
    const { privateKey, publicKey } = generateKeyPairSync("rsa", {
      modulusLength: 2048,
      privateKeyEncoding: { format: "pem", type: "pkcs1" },
      publicKeyEncoding: { format: "pem", type: "pkcs1" },
    });

    const url = new URL("https://api.audible.com/1.0/library?page=2");
    const headers = await signAudibleRequest({
      bodyText: "{\"hello\":\"world\"}",
      credentials: {
        adpToken: "adp-token",
        privateKeyPem: privateKey,
      },
      headers: new Headers(),
      method: "POST",
      url,
    });

    expect(headers["x-adp-alg"]).toBe("SHA256withRSA:1.0");
    expect(headers["x-adp-token"]).toBe("adp-token");

    const signatureHeader = headers["x-adp-signature"];
    if (!signatureHeader) {
      throw new Error("Missing x-adp-signature header");
    }
    const separatorIndex = signatureHeader.indexOf(":");
    const encodedSignature = signatureHeader.slice(0, separatorIndex);
    const isoDate = signatureHeader.slice(separatorIndex + 1);
    const data = `POST\n/1.0/library?page=2\n${isoDate}\n{"hello":"world"}\nadp-token`;
    const valid = verify("RSA-SHA256", Buffer.from(data), publicKey, Buffer.from(encodedSignature, "base64"));
    expect(valid).toBe(true);
  });
});
