import http from "node:http";
import { readFile } from "node:fs/promises";
import { extname, join, normalize } from "node:path";
import { fileURLToPath } from "node:url";

const root = fileURLToPath(new URL("./dist/", import.meta.url));
const port = Number(process.env.PORT || 4173);

const mimeTypes = {
  ".html": "text/html; charset=utf-8",
  ".js": "text/javascript; charset=utf-8",
  ".mjs": "text/javascript; charset=utf-8",
  ".css": "text/css; charset=utf-8",
  ".json": "application/json",
  ".svg": "image/svg+xml",
  ".png": "image/png",
  ".ico": "image/x-icon",
};

const server = http.createServer(async (req, res) => {
  const urlPath = decodeURIComponent(new URL(req.url, `http://localhost:${port}`).pathname);

  if (urlPath.endsWith(".map")) {
    res.writeHead(404, { "Content-Type": "text/plain" });
    res.end("source maps are not served publicly");
    return;
  }

  const safePath = normalize(urlPath).replace(/^(\.\.[/\\])+/, "");
  let filePath = join(root, safePath === "/" ? "index.html" : safePath);

  try {
    const content = await readFile(filePath);
    res.writeHead(200, { "Content-Type": mimeTypes[extname(filePath)] || "application/octet-stream" });
    res.end(content);
  } catch {
    try {
      const fallback = await readFile(join(root, "index.html"));
      res.writeHead(200, { "Content-Type": mimeTypes[".html"] });
      res.end(fallback);
    } catch {
      res.writeHead(404, { "Content-Type": "text/plain" });
      res.end("not found — run `npm run build` first");
    }
  }
});

server.listen(port, () => {
  console.log(`serving dist/ at http://localhost:${port} (.map requests return 404)`);
});
