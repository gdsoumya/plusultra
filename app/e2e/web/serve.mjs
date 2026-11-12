// Serves the single-page web export with a fallback to index.html, which `expo serve` lacks, so
// deep links such as /workout/<id> load. Usage: node e2e/web/serve.mjs [port]
import { createReadStream, existsSync, statSync } from "node:fs";
import { createServer } from "node:http";
import { extname, join, normalize } from "node:path";

const root = new URL("../../dist/", import.meta.url).pathname;
const types = {
  ".html": "text/html",
  ".js": "text/javascript",
  ".css": "text/css",
  ".json": "application/json",
  ".ico": "image/x-icon",
  ".png": "image/png",
  ".ttf": "font/ttf",
};

createServer((req, res) => {
  const path = normalize(decodeURIComponent(new URL(req.url, "http://x").pathname));
  let file = join(root, path);
  if (!file.startsWith(root) || !existsSync(file) || statSync(file).isDirectory())
    file = join(root, "index.html");
  res.setHeader("Content-Type", types[extname(file)] ?? "application/octet-stream");
  createReadStream(file).pipe(res);
}).listen(Number(process.argv[2] ?? 8082));
