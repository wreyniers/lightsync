/**
 * Single source of truth for the app semver: build/config.yml → info.version
 */
import fs from "fs";
import path from "path";
import { fileURLToPath } from "url";

const __dirname = path.dirname(fileURLToPath(import.meta.url));

export function getConfigPath() {
  return path.join(__dirname, "..", "build", "config.yml");
}

/** @returns {string} e.g. "1.1.1" */
export function getAppVersion() {
  const text = fs.readFileSync(getConfigPath(), "utf8");
  const parts = text.split(/^info:\s*$/m);
  if (parts.length < 2) {
    throw new Error("Could not find info: block in build/config.yml");
  }
  const infoBlock = parts[1];
  const m = infoBlock.match(/^\s+version:\s*["']([^"']+)["']/m);
  if (!m) {
    throw new Error("Could not find info.version in build/config.yml");
  }
  return m[1];
}

/** Semver → Windows assembly four-part version (e.g. 1.2.3 → 1.2.3.0). */
export function semverToWinAssemblyVersion(semver) {
  const core = semver.split("-")[0].trim();
  const nums = core.split(".").map((x) => parseInt(x, 10) || 0);
  while (nums.length < 4) nums.push(0);
  return nums.slice(0, 4).join(".");
}
