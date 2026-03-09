import { spawn } from "node:child_process";

export async function openBrowser(url: string): Promise<void> {
  const platform = process.platform;
  if (platform === "win32") {
    const escapedUrl = url.replace(/'/g, "''");
    spawn(
      "powershell",
      ["-NoProfile", "-Command", `Start-Process '${escapedUrl}'`],
      { detached: true, stdio: "ignore" },
    ).unref();
    return;
  }

  if (platform === "darwin") {
    spawn("open", [url], { detached: true, stdio: "ignore" }).unref();
    return;
  }

  spawn("xdg-open", [url], { detached: true, stdio: "ignore" }).unref();
}
