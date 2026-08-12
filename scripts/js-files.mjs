import { readdir } from "node:fs/promises";
import { join } from "node:path";

export async function javascriptFiles(root = "web/static") {
  const files = [];
  async function visit(directory) {
    const entries = await readdir(directory, { withFileTypes: true });
    for (const entry of entries) {
      const path = join(directory, entry.name);
      if (entry.isDirectory()) {
        await visit(path);
      } else if (entry.isFile() && entry.name.endsWith(".js")) {
        files.push(path);
      }
    }
  }
  await visit(root);
  return files.sort();
}
