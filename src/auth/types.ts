export type AuthStrategyKind = "signed";

export interface AudibleRequestContext {
  method: string;
  url: URL;
  headers: Headers;
  bodyText?: string;
}

export interface AuthStrategy {
  readonly kind: AuthStrategyKind;
  readonly name: string;
  apply(context: AudibleRequestContext): Promise<void>;
}

export interface SignedCredentials {
  adpToken: string;
  privateKeyPem: string;
}

export interface SignedRequestContext extends AudibleRequestContext {
  credentials: SignedCredentials;
}

export type SignedRequestSigner = (
  context: SignedRequestContext,
) => Promise<Record<string, string>> | Record<string, string>;
