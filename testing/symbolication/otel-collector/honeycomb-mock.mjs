import http from "node:http";
import { mkdirSync, writeFileSync } from "node:fs";
import { join } from "node:path";
import { fileURLToPath } from "node:url";
import { gunzipSync } from "node:zlib";

const port = Number(process.env.PORT || 8090);
const captureDir = fileURLToPath(new URL("./captures/honeycomb/", import.meta.url));
mkdirSync(captureDir, { recursive: true });

let counter = 0;

const server = http.createServer((req, res) => {
  const chunks = [];
  req.on("data", (chunk) => chunks.push(chunk));
  req.on("end", () => {
    counter += 1;
    let body = Buffer.concat(chunks);
    if (req.headers["content-encoding"] === "gzip") {
      body = gunzipSync(body);
    }
    const contentType = req.headers["content-type"] || "unknown";
    const ext = contentType.includes("json") ? "json" : "bin";
    const signal = req.url.replaceAll("/", "_");
    const file = join(captureDir, `${String(counter).padStart(4, "0")}${signal}.${ext}`);
    writeFileSync(file, body);
    console.log(`[honeycomb-mock] #${counter} ${req.method} ${req.url} (${contentType}, ${body.length} bytes) -> ${file}`);
    res.writeHead(200, { "Content-Type": "application/json" });
    res.end("{}");
  });
});

server.listen(port, () => {
  console.log(`[honeycomb-mock] listening on http://localhost:${port}, capturing to ${captureDir}`);
});
