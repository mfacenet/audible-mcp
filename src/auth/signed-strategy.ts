import type {
  AudibleRequestContext,
  AuthStrategy,
  SignedCredentials,
  SignedRequestSigner,
} from "./types.js";

export class SignedAuthStrategy implements AuthStrategy {
  readonly kind = "signed";
  readonly name = "signed";

  constructor(
    private readonly credentials: SignedCredentials,
    private readonly signer: SignedRequestSigner,
  ) {}

  async apply(context: AudibleRequestContext): Promise<void> {
    context.headers.set("x-adp-token", this.credentials.adpToken);
    const signedHeaders = await this.signer({
      ...context,
      credentials: this.credentials,
    });

    for (const [name, value] of Object.entries(signedHeaders)) {
      context.headers.set(name, value);
    }
  }
}
