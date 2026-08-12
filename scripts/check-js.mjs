import { spawnSync } from "node:child_process";
import { javascriptFiles } from "./js-files.mjs";

let failed = false;
for (const file of await javascriptFiles()) {
  const result = spawnSync(process.execPath, ["--check", file], { encoding: "utf8" });
  if (result.status !== 0) {
    failed = true;
    process.stderr.write(result.stderr || `${file}: syntax check failed\n`);
  }
}
if (failed) {
  process.exitCode = 1;
}
