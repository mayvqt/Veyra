import assert from "node:assert/strict";
import { readFile } from "node:fs/promises";
import test from "node:test";
import { javascriptFiles } from "../js-files.mjs";

test("browser scripts avoid unsafe dynamic code and HTML sinks", async () => {
  for (const file of await javascriptFiles()) {
    const source = await readFile(file, "utf8");
    assert.doesNotMatch(source, /\b(?:eval|Function)\s*\(/, `${file} uses dynamic code evaluation`);
    assert.doesNotMatch(source, /\.(?:innerHTML|outerHTML)\s*=/, `${file} writes untrusted HTML`);
    assert.doesNotMatch(source, /document\.write\s*\(/, `${file} uses document.write`);
    if (file.endsWith("web/static/js/dashboard/request_bot.js")) {
      assert.match(source, /searchSequence/);
      assert.match(source, /requestSequence !== searchSequence/);
      assert.match(source, /AbortController/);
      assert.match(source, /document\.createElement\("fieldset"\)/);
      assert.equal((source.match(/document\.createElement\("label"\)/g) || []).length, 1, "season controls should only label each individual checkbox");
    }
  }
});
