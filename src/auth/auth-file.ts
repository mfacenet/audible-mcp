import { mkdir, readFile, writeFile } from "node:fs/promises";
import path from "node:path";

export interface AudibleAuthFile {
  accessToken: string;
  adpToken: string;
  customerInfo?: Record<string, unknown>;
  deviceInfo?: Record<string, unknown>;
  devicePrivateKey: string;
  domain: string;
  expiresAt: number;
  locale: string;
  refreshToken: string;
  serial: string;
  storeAuthenticationCookie?: Record<string, unknown>;
  websiteCookies?: Record<string, string>;
  withUsername: boolean;
}

export async function loadAuthFile(filename: string): Promise<AudibleAuthFile> {
  const content = await readFile(filename, "utf8");
  return JSON.parse(content) as AudibleAuthFile;
}

export async function saveAuthFile(filename: string, data: AudibleAuthFile): Promise<void> {
  const absolutePath = path.resolve(filename);
  await mkdir(path.dirname(absolutePath), { recursive: true });
  await writeFile(absolutePath, `${JSON.stringify(data, null, 2)}\n`, "utf8");
}

