import { describe, expect, it } from "vitest";
import path from "node:path";

describe("smoke test script", () => {
  it("resolves the default auth file location in the repo root", () => {
    const resolved = path.resolve(process.cwd(), "audible-auth.json");
    expect(resolved.endsWith(path.join("audible-mcp", "audible-auth.json"))).toBe(true);
  });
});
