/**
 * Propagates build/config.yml info.version into files that must exist on disk
 * (npm metadata, Windows syso inputs). Run before production frontend builds.
 */
import fs from "fs";
import path from "path";
import { execSync } from "child_process";
import { fileURLToPath } from "url";
import { getAppVersion, semverToWinAssemblyVersion } from "./app-version.mjs";

const __dirname = path.dirname(fileURLToPath(import.meta.url));
const root = path.join(__dirname, "..");

function syncNpmVersion(v) {
  const frontendDir = path.join(root, "frontend");
  const pkgPath = path.join(frontendDir, "package.json");
  const pkg = JSON.parse(fs.readFileSync(pkgPath, "utf8"));
  if (pkg.version === v) return;
  execSync(`npm version ${v} --no-git-tag-version`, {
    cwd: frontendDir,
    stdio: "inherit",
    shell: true,
  });
}

function syncInfoJson(v) {
  const p = path.join(root, "build", "windows", "info.json");
  const data = JSON.parse(fs.readFileSync(p, "utf8"));
  data.fixed.file_version = v;
  const info0000 = data.info?.["0000"];
  if (info0000) info0000.ProductVersion = v;
  fs.writeFileSync(p, JSON.stringify(data, null, "\t") + "\n", "utf8");
}

function syncManifest(winVer) {
  const p = path.join(root, "build", "windows", "wails.exe.manifest");
  let s = fs.readFileSync(p, "utf8");
  s = s.replace(
    /(<assemblyIdentity version=")([^"]+)(" processorArchitecture="\*" name="LightSync")/,
    `$1${winVer}$3`
  );
  fs.writeFileSync(p, s, "utf8");
}

const v = getAppVersion();
const winAsm = semverToWinAssemblyVersion(v);
syncNpmVersion(v);
syncInfoJson(v);
syncManifest(winAsm);
console.log(`sync-version: ${v} (assembly ${winAsm})`);
