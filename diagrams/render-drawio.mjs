// Renders .drawio files to PNG with the draw.io viewer in headless Chromium, for machines without the draw.io
// desktop app. Needs the `playwright` package with Chromium installed, and a local copy of the viewer:
//
//   curl -o /tmp/viewer.js https://viewer.diagrams.net/js/viewer-static.min.js
//   node diagrams/render-drawio.mjs /tmp/viewer.js <out-dir> diagrams/docs/rum/*.drawio
//
// The PNG is 2x and as wide as the viewer's canvas; trim it to the drawing with ImageMagick:
//   magick in.png -trim +repage -bordercolor '#FFFFFF' -border 28 out.png
//
// diagrams/docs/rum (light, docs/public/rum/*.png) and diagrams/website/rum (dark, website/public/images/rum-*.png)
// hold the same three diagrams in two themes; change both when one changes.
import { chromium } from "playwright";
import { readFileSync, writeFileSync } from "node:fs";
import { basename, join } from "node:path";

const [viewerPath, outDir, ...files] = process.argv.slice(2);
const viewer = readFileSync(viewerPath, "utf8");
const browser = await chromium.launch();
for (const file of files) {
  const xml = readFileSync(file, "utf8");
  const bg = /background="([^"]+)"/.exec(xml)?.[1] ?? "#FFFFFF";
  const page = await browser.newPage({ viewport: { width: 1400, height: 1200 }, deviceScaleFactor: 2 });
  await page.setContent(
    `<!doctype html><html><body style="margin:0;background:${bg}">
     <div class="mxgraph" id="g" style="width:1380px"></div></body></html>`,
  );
  await page.evaluate(
    (c) => document.getElementById("g").setAttribute("data-mxgraph", c),
    JSON.stringify({ xml, toolbar: null, nav: false, resize: false, lightbox: false }),
  );
  await page.addScriptTag({ content: viewer });
  await page.evaluate(() => window.GraphViewer.createViewerForElement(document.getElementById("g")));
  await page.waitForSelector("#g svg", { timeout: 30000, state: "attached" });
  await page.waitForTimeout(500);
  const out = join(outDir, basename(file).replace(/\.drawio$/, ".png"));
  const r = await page.evaluate(() => { const v = document.querySelector("#g svg").getBoundingClientRect(); const b = document.querySelector("#g svg > g").getBoundingClientRect(); return { x: b.x, y: v.y, width: b.width, height: v.height }; });
  const pad = 12;
  await page.setViewportSize({ width: 1400, height: Math.ceil(r.y + r.height + pad + 20) });
  writeFileSync(out, await page.screenshot({ clip: { x: Math.max(0, r.x - pad), y: Math.max(0, r.y - pad), width: r.width + 2 * pad, height: r.height + 2 * pad } }));
  console.log(out);
  await page.close();
}
await browser.close();
