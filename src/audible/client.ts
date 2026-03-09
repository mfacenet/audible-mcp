import type { AuthStrategy } from "../auth/types.js";

export interface AudibleClientOptions {
  auth: AuthStrategy;
  baseUrl: string;
  fetchFn?: typeof fetch;
  userAgent?: string;
}

export interface AudibleRequestOptions {
  body?: BodyInit | object;
  headers?: HeadersInit;
  method?: string;
  query?: Record<string, string | number | boolean | undefined>;
}

export interface AudibleResponse {
  bodyText: string;
  headers: Headers;
  ok: boolean;
  status: number;
  url: string;
}

const DEFAULT_USER_AGENT = "audible-mcp-research/0.1";

export class AudibleClient {
  private readonly auth: AuthStrategy;
  private readonly baseUrl: URL;
  private readonly fetchFn: typeof fetch;
  private readonly userAgent: string;

  constructor(options: AudibleClientOptions) {
    this.auth = options.auth;
    this.baseUrl = new URL(options.baseUrl);
    this.fetchFn = options.fetchFn ?? fetch;
    this.userAgent = options.userAgent ?? DEFAULT_USER_AGENT;
  }

  async request(pathname: string, options: AudibleRequestOptions = {}): Promise<AudibleResponse> {
    const method = options.method ?? "GET";
    const url = new URL(pathname, this.baseUrl);

    for (const [key, value] of Object.entries(options.query ?? {})) {
      if (value !== undefined) {
        url.searchParams.set(key, String(value));
      }
    }

    const headers = new Headers(options.headers);
    headers.set("accept", "application/json");
    headers.set("user-agent", this.userAgent);

    let body: BodyInit | undefined;
    let bodyText: string | undefined;

    if (options.body !== undefined) {
      if (
        typeof options.body === "string" ||
        options.body instanceof Blob ||
        options.body instanceof ArrayBuffer ||
        ArrayBuffer.isView(options.body) ||
        options.body instanceof FormData ||
        options.body instanceof URLSearchParams ||
        options.body instanceof ReadableStream
      ) {
        body = options.body as BodyInit;
        bodyText = typeof options.body === "string" ? options.body : undefined;
      } else {
        bodyText = JSON.stringify(options.body);
        body = bodyText;
        headers.set("content-type", "application/json");
      }
    }

    const requestContext =
      bodyText === undefined
        ? { headers, method, url }
        : { bodyText, headers, method, url };

    await this.auth.apply(requestContext);

    const requestInit: RequestInit = {
      headers,
      method,
      redirect: "manual",
    };

    if (body !== undefined) {
      requestInit.body = body;
    }

    const response = await this.fetchFn(url, requestInit);

    return {
      bodyText: await response.text(),
      headers: response.headers,
      ok: response.ok,
      status: response.status,
      url: response.url,
    };
  }
}
