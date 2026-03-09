import { createHash, createSign, randomBytes, randomUUID } from "node:crypto";
import { URLSearchParams } from "node:url";
import type { AudibleMarketplace } from "../audible/marketplaces.js";
import type { AudibleAuthFile } from "./auth-file.js";
import type { SignedRequestSigner } from "./types.js";

export interface ExternalLoginSession {
  codeVerifier: string;
  loginUrl: string;
  marketplace: AudibleMarketplace;
  serial: string;
  withUsername: boolean;
}

export interface AudibleRegistrationResult {
  accessToken: string;
  adpToken: string;
  customerInfo?: Record<string, unknown>;
  deviceInfo?: Record<string, unknown>;
  devicePrivateKey: string;
  expiresAt: number;
  refreshToken: string;
  storeAuthenticationCookie?: Record<string, unknown>;
  websiteCookies?: Record<string, string>;
}

interface RegisterResponseTokenCookie {
  Name: string;
  Value: string;
}

interface RegisterResponseShape {
  response?: {
    success?: {
      extensions?: {
        customer_info?: Record<string, unknown>;
        device_info?: Record<string, unknown>;
      };
      tokens?: {
        bearer?: {
          access_token: string;
          expires_in: string | number;
          refresh_token: string;
        };
        mac_dms?: {
          adp_token: string;
          device_private_key: string;
        };
        store_authentication_cookie?: Record<string, unknown>;
        website_cookies?: RegisterResponseTokenCookie[];
      };
    };
  };
}

interface RefreshAccessTokenResponseShape {
  access_token: string;
  expires_in: string | number;
}

interface RefreshCookieResponseShape {
  response?: {
    tokens?: {
      cookies?: Record<string, RegisterResponseTokenCookie[]>;
    };
  };
}

function createCodeVerifier(length = 32): string {
  return randomBytes(length).toString("base64url");
}

function createS256CodeChallenge(verifier: string): string {
  return createHash("sha256").update(verifier).digest("base64url");
}

export function buildDeviceSerial(): string {
  return randomUUID().replace(/-/g, "").toUpperCase();
}

export function buildClientId(serial: string): string {
  return Buffer.from(`${serial}#A2CZJZGLK2JJVM`, "utf8").toString("hex");
}

export function createExternalLoginSession(
  marketplace: AudibleMarketplace,
  options: { serial?: string; withUsername?: boolean } = {},
): ExternalLoginSession {
  const withUsername = options.withUsername ?? false;
  if (withUsername && !["com", "co.uk", "de"].includes(marketplace.domain)) {
    throw new Error("Login with username is only supported for US, UK, and DE marketplaces.");
  }

  const serial = options.serial ?? buildDeviceSerial();
  const codeVerifier = createCodeVerifier();
  const clientId = buildClientId(serial);
  const codeChallenge = createS256CodeChallenge(codeVerifier);

  const baseHost = withUsername ? `https://www.audible.${marketplace.domain}` : `https://www.amazon.${marketplace.domain}`;
  const returnTo = `${baseHost}/ap/maplanding`;
  const assocHandle = withUsername
    ? `amzn_audible_ios_lap_${marketplace.code}`
    : `amzn_audible_ios_${marketplace.code}`;
  const pageId = withUsername ? "amzn_audible_ios_privatepool" : "amzn_audible_ios";

  const params = new URLSearchParams({
    "accountStatusPolicy": "P1",
    "forceMobileLayout": "true",
    "marketPlaceId": marketplace.marketPlaceId,
    "openid.assoc_handle": assocHandle,
    "openid.claimed_id": "http://specs.openid.net/auth/2.0/identifier_select",
    "openid.identity": "http://specs.openid.net/auth/2.0/identifier_select",
    "openid.mode": "checkid_setup",
    "openid.ns": "http://specs.openid.net/auth/2.0",
    "openid.ns.oa2": "http://www.amazon.com/ap/ext/oauth/2",
    "openid.ns.pape": "http://specs.openid.net/extensions/pape/1.0",
    "openid.oa2.client_id": `device:${clientId}`,
    "openid.oa2.code_challenge": codeChallenge,
    "openid.oa2.code_challenge_method": "S256",
    "openid.oa2.response_type": "code",
    "openid.oa2.scope": "device_auth_access",
    "openid.pape.max_auth_age": "0",
    "openid.return_to": returnTo,
    "pageId": pageId,
  });

  return {
    codeVerifier,
    loginUrl: `${baseHost}/ap/signin?${params.toString()}`,
    marketplace,
    serial,
    withUsername,
  };
}

export function extractAuthorizationCode(responseUrl: string): string {
  const url = new URL(responseUrl);
  const authorizationCode = url.searchParams.get("openid.oa2.authorization_code");
  if (!authorizationCode) {
    throw new Error("Response URL does not contain openid.oa2.authorization_code.");
  }
  return authorizationCode;
}

function computeExpiryEpoch(expiresInSeconds: string | number): number {
  return Date.now() + Number(expiresInSeconds) * 1000;
}

function normalizeWebsiteCookies(
  cookies: RegisterResponseTokenCookie[] | Record<string, RegisterResponseTokenCookie[]> | undefined,
): Record<string, string> | undefined {
  if (!cookies) {
    return undefined;
  }

  const normalized: Record<string, string> = {};
  const cookieList = Array.isArray(cookies) ? cookies : Object.values(cookies).flat();
  for (const cookie of cookieList) {
    normalized[cookie.Name] = cookie.Value.replaceAll('"', "");
  }
  return normalized;
}

async function parseJsonResponse<T>(response: Response): Promise<T> {
  const bodyText = await response.text();
  try {
    return JSON.parse(bodyText) as T;
  } catch {
    throw new Error(`Expected JSON response but received: ${bodyText.slice(0, 240)}`);
  }
}

export async function registerAudibleDevice(
  session: ExternalLoginSession,
  authorizationCode: string,
  fetchFn: typeof fetch = fetch,
): Promise<AudibleRegistrationResult> {
  const body = {
    auth_data: {
      authorization_code: authorizationCode,
      client_domain: "DeviceLegacy",
      client_id: buildClientId(session.serial),
      code_algorithm: "SHA-256",
      code_verifier: session.codeVerifier,
    },
    cookies: {
      domain: `.amazon.${session.marketplace.domain}`,
      website_cookies: [],
    },
    registration_data: {
      app_name: "Audible",
      app_version: "3.56.2",
      device_model: "iPhone",
      device_name: "%F IRST_NAME%%FIRST_NAME_POSSESSIVE_STRING%%DUPE_STRATEGY_1ST%Audible for iPhone",
      device_serial: session.serial,
      device_type: "A2CZJZGLK2JJVM",
      domain: "Device",
      os_version: "15.0.0",
      software_version: "35602678",
    },
    requested_extensions: ["device_info", "customer_info"],
    requested_token_type: ["bearer", "mac_dms", "website_cookies", "store_authentication_cookie"],
  };

  const targetDomain = session.withUsername ? "audible" : "amazon";
  const response = await fetchFn(`https://api.${targetDomain}.${session.marketplace.domain}/auth/register`, {
    body: JSON.stringify(body),
    headers: {
      "content-type": "application/json",
    },
    method: "POST",
  });

  const data = await parseJsonResponse<RegisterResponseShape>(response);
  if (!response.ok) {
    throw new Error(`Device registration failed with ${response.status}: ${JSON.stringify(data)}`);
  }

  const success = data.response?.success;
  const tokens = success?.tokens;
  const bearer = tokens?.bearer;
  const macDms = tokens?.mac_dms;
  const websiteCookies = normalizeWebsiteCookies(tokens?.website_cookies);

  if (!success || !tokens || !bearer || !macDms) {
    throw new Error("Device registration response was missing required fields.");
  }

  return {
    accessToken: bearer.access_token,
    adpToken: macDms.adp_token,
    devicePrivateKey: macDms.device_private_key,
    expiresAt: computeExpiryEpoch(bearer.expires_in),
    refreshToken: bearer.refresh_token,
    ...(success.extensions?.customer_info
      ? { customerInfo: success.extensions.customer_info }
      : {}),
    ...(success.extensions?.device_info ? { deviceInfo: success.extensions.device_info } : {}),
    ...(tokens.store_authentication_cookie
      ? { storeAuthenticationCookie: tokens.store_authentication_cookie }
      : {}),
    ...(websiteCookies ? { websiteCookies } : {}),
  };
}

export async function refreshAudibleAccessToken(
  auth: Pick<AudibleAuthFile, "domain" | "refreshToken" | "withUsername">,
  fetchFn: typeof fetch = fetch,
): Promise<Pick<AudibleAuthFile, "accessToken" | "expiresAt">> {
  const targetDomain = auth.withUsername ? "audible" : "amazon";
  const body = new URLSearchParams({
    app_name: "Audible",
    app_version: "3.56.2",
    requested_token_type: "access_token",
    source_token: auth.refreshToken,
    source_token_type: "refresh_token",
  });

  const response = await fetchFn(`https://api.${targetDomain}.${auth.domain}/auth/token`, {
    body,
    method: "POST",
  });
  const data = await parseJsonResponse<RefreshAccessTokenResponseShape>(response);
  if (!response.ok) {
    throw new Error(`Access token refresh failed with ${response.status}: ${JSON.stringify(data)}`);
  }

  return {
    accessToken: data.access_token,
    expiresAt: computeExpiryEpoch(data.expires_in),
  };
}

export async function refreshAudibleWebsiteCookies(
  auth: Pick<AudibleAuthFile, "domain" | "refreshToken" | "withUsername">,
  cookiesDomain: string,
  fetchFn: typeof fetch = fetch,
): Promise<Record<string, string>> {
  const targetDomain = auth.withUsername ? "audible" : "amazon";
  const body = new URLSearchParams({
    app_name: "Audible",
    app_version: "3.56.2",
    domain: `.${targetDomain}.${cookiesDomain}`,
    requested_token_type: "auth_cookies",
    source_token: auth.refreshToken,
    source_token_type: "refresh_token",
  });

  const response = await fetchFn(`https://www.${targetDomain}.${auth.domain}/ap/exchangetoken/cookies`, {
    body,
    method: "POST",
  });
  const data = await parseJsonResponse<RefreshCookieResponseShape>(response);
  if (!response.ok) {
    throw new Error(`Website cookie refresh failed with ${response.status}: ${JSON.stringify(data)}`);
  }

  const cookies = normalizeWebsiteCookies(data.response?.tokens?.cookies);
  return cookies ?? {};
}

export function createAudibleAuthFile(
  session: ExternalLoginSession,
  registration: AudibleRegistrationResult,
): AudibleAuthFile {
  return {
    accessToken: registration.accessToken,
    adpToken: registration.adpToken,
    devicePrivateKey: registration.devicePrivateKey,
    domain: session.marketplace.domain,
    expiresAt: registration.expiresAt,
    locale: session.marketplace.code,
    refreshToken: registration.refreshToken,
    serial: session.serial,
    withUsername: session.withUsername,
    ...(registration.customerInfo ? { customerInfo: registration.customerInfo } : {}),
    ...(registration.deviceInfo ? { deviceInfo: registration.deviceInfo } : {}),
    ...(registration.storeAuthenticationCookie
      ? { storeAuthenticationCookie: registration.storeAuthenticationCookie }
      : {}),
    ...(registration.websiteCookies ? { websiteCookies: registration.websiteCookies } : {}),
  };
}

export const signAudibleRequest: SignedRequestSigner = async (context) => {
  const body = context.bodyText ?? "";
  const date = new Date().toISOString();
  const data = `${context.method}\n${context.url.pathname}${context.url.search}\n${date}\n${body}\n${context.credentials.adpToken}`;
  const signature = createSign("RSA-SHA256")
    .update(data)
    .end()
    .sign(context.credentials.privateKeyPem, "base64");

  return {
    "x-adp-alg": "SHA256withRSA:1.0",
    "x-adp-signature": `${signature}:${date}`,
    "x-adp-token": context.credentials.adpToken,
  };
};
