// Two-panel comparison of the restaurant analogy and the Go service, used as the
// hero image and the OG card. Dark palette from website/app/globals.css, titles
// in JetBrains Mono and body in Inter like the site itself. Render:
//   npm i playwright && node restaurant-vs-go-service.mjs
// Both fonts are embedded as subsetted woff2 (Inter is the variable font, so one
// file covers every weight), so the SVG renders the same everywhere without
// fetching anything. Output goes to
// website/public/blog/reliable-apps-crash-so-why-does-go-recover-panics/.
import { chromium } from "playwright";
import fs from "node:fs";
import path from "node:path";
import { fileURLToPath } from "node:url";

const HERE = path.dirname(fileURLToPath(import.meta.url));
const OUT_DIR = path.resolve(HERE, "../../../website/public/blog/reliable-apps-crash-so-why-does-go-recover-panics");
const SVG_NAME = process.argv[2] ?? "restaurant-vs-go-service.svg";
const PNG_NAME = process.argv[3] ?? "og.png";

const inter = fs.readFileSync(path.join(HERE, "fonts/inter.sub.woff2")).toString("base64");
const mono = fs.readFileSync(path.join(HERE, "fonts/jetbrains-mono-700.sub.woff2")).toString("base64");

const spec = {
  width: 1200,
  height: 630,
  bg: "#0a0d14",
  eyebrow: "RELIABLE APPS CRASH. THE GOOD ONES COME BACK CLEAN.",
  panel: { fill: "#10151f", stroke: "#283041" },
  divider: "#283041",
  arrow: "#5a6374",
  fg0: "#f4f6fb",
  fg1: "#c9d0dd",
  fg2: "#8a93a6",
  fg3: "#5a6374",
  panels: [
    {
      x: 44, y: 60, w: 548, h: 452, rx: 18,
      pill: { fill: "#241b16", stroke: "#5a3d2a", text: "#ffcfa8" },
      title: "Restaurant",
      subtitle: "The fridge broke and stayed warm all night.",
      steps: [
        { color: "red", lines: ["Close the doors, throw out", "everything you can't trust"] },
        { color: "blue", lines: ["Restock from a supplier you trust"] },
        { color: "green", lines: ["Seat guests again"] },
      ],
    },
    {
      x: 608, y: 60, w: 548, h: 452, rx: 18,
      pill: { fill: "#141d30", stroke: "#2c4a7a", text: "#aecbff" },
      title: "Go service",
      subtitle: "A panic landed between two writes.",
      steps: [
        { color: "red", lines: ["exit(1), nothing in memory survives"] },
        { color: "blue", lines: ["The supervisor starts a clean process", "that rebuilds from durable state"] },
        { color: "green", lines: ["Readiness passes, traffic comes back"] },
      ],
    },
  ],
  colors: {
    red: { fill: "#1b1115", stroke: "#ff5a5f", text: "#ffb1b3" },
    blue: { fill: "#0d1a26", stroke: "#00d4ff", text: "#9fe6ff" },
    green: { fill: "#0c1714", stroke: "#22e0a8", text: "#7ff0c8" },
  },
  legend: [
    { color: "red", label: "Stop and discard what you can't trust" },
    { color: "blue", label: "Rebuild from a source you trust" },
    { color: "green", label: "Serve again, invariants hold" },
  ],
};

const SANS = "Inter, -apple-system, 'Segoe UI', Roboto, Helvetica, Arial, sans-serif";
const MONO = "'JetBrains Mono', ui-monospace, SFMono-Regular, Menlo, Consolas, monospace";
const fontFaces =
  `@font-face{font-family:'Inter';font-weight:100 900;font-style:normal;src:url(data:font/woff2;base64,${inter}) format('woff2')}` +
  `@font-face{font-family:'JetBrains Mono';font-weight:700;font-style:normal;src:url(data:font/woff2;base64,${mono}) format('woff2')}`;

const browser = await chromium.launch();
const page = await browser.newPage({ viewport: { width: spec.width, height: spec.height }, deviceScaleFactor: 1 });
await page.setContent(`<!doctype html><html><head><style>${fontFaces}html,body{margin:0;background:${spec.bg}}</style></head><body></body></html>`);
await page.evaluate(() => document.fonts.ready);

const svgMarkup = await page.evaluate(async ({ spec, fontFaces, SANS, MONO }) => {
  const NS = "http://www.w3.org/2000/svg";
  const svg = document.createElementNS(NS, "svg");
  svg.setAttribute("xmlns", NS);
  svg.setAttribute("viewBox", `0 0 ${spec.width} ${spec.height}`);
  svg.setAttribute("width", spec.width);
  svg.setAttribute("height", spec.height);
  svg.setAttribute("role", "img");
  svg.setAttribute("font-family", SANS);
  document.body.appendChild(svg);

  const el = (name, attrs, parent = svg) => {
    const e = document.createElementNS(NS, name);
    for (const [k, v] of Object.entries(attrs)) e.setAttribute(k, v);
    parent.appendChild(e);
    return e;
  };
  const text = (x, y, str, { size = 22, weight = 500, fill = spec.fg1, anchor = "middle", family, spacing } = {}, parent = svg) => {
    const attrs = { x, y, "font-size": size, "font-weight": weight, fill, "text-anchor": anchor };
    if (family) attrs["font-family"] = family;
    if (spacing) attrs["letter-spacing"] = spacing;
    const t = el("text", attrs, parent);
    t.textContent = str;
    return t;
  };
  const width = (t) => t.getComputedTextLength();

  const defs = el("defs", {});
  const style = document.createElementNS(NS, "style");
  style.textContent = fontFaces;
  defs.appendChild(style);
  const marker = el("marker", { id: "arrow", viewBox: "0 0 10 10", refX: 8, refY: 5, markerWidth: 7, markerHeight: 7, orient: "auto-start-reverse" }, defs);
  el("path", { d: "M 0 0 L 10 5 L 0 10 z", fill: spec.arrow }, marker);

  el("rect", { x: 0, y: 0, width: spec.width, height: spec.height, fill: spec.bg });
  text(44, 40, spec.eyebrow, { size: 12, weight: 700, fill: spec.fg3, anchor: "start", family: MONO, spacing: 1.6 });

  const dividerX = (spec.panels[0].x + spec.panels[0].w + spec.panels[1].x) / 2;
  el("line", { x1: dividerX, y1: spec.panels[0].y + 24, x2: dividerX, y2: spec.panels[0].y + spec.panels[0].h - 24, stroke: spec.divider, "stroke-width": 1.5 });

  for (const p of spec.panels) {
    const g = el("g", {});
    el("rect", { x: p.x, y: p.y, width: p.w, height: p.h, rx: p.rx, fill: spec.panel.fill, stroke: spec.panel.stroke, "stroke-width": 1.5 }, g);
    const cx = p.x + p.w / 2;

    // title pill sized to the text
    const title = text(cx, p.y + 57, p.title, { size: 28, weight: 700, fill: p.pill.text, family: MONO }, g);
    const tw = width(title);
    const pill = el("rect", { x: cx - tw / 2 - 28, y: p.y + 20, width: tw + 56, height: 54, rx: 27, fill: p.pill.fill, stroke: p.pill.stroke, "stroke-width": 1.5 }, g);
    g.insertBefore(pill, title);

    text(cx, p.y + 114, p.subtitle, { size: 21, fill: spec.fg2 }, g);

    const boxW = 484, boxH = 76, gap = 32;
    const bx = cx - boxW / 2;
    let by = p.y + 138;
    p.steps.forEach((s, i) => {
      const c = spec.colors[s.color];
      el("rect", { x: bx, y: by, width: boxW, height: boxH, rx: 12, fill: c.fill, stroke: c.stroke, "stroke-width": 1.5 }, g);
      const lh = 27;
      const startY = by + boxH / 2 - ((s.lines.length - 1) * lh) / 2 + 8;
      s.lines.forEach((line, li) => text(cx, startY + li * lh, line, { size: 22, fill: c.text }, g));
      if (i < p.steps.length - 1) {
        el("line", { x1: cx, y1: by + boxH + 4, x2: cx, y2: by + boxH + gap - 6, stroke: spec.arrow, "stroke-width": 1.6, "marker-end": "url(#arrow)" }, g);
      }
      by += boxH + gap;
    });
  }

  // legend: measure labels, then lay out centered
  const legendY = 572;
  const sw = 20, swGap = 12, itemGap = 48;
  const items = spec.legend.map((l) => {
    const t = text(0, legendY + 7, l.label, { size: 19, fill: spec.fg1, anchor: "start" });
    return { ...l, t, w: sw + swGap + width(t) };
  });
  const total = items.reduce((a, it) => a + it.w, 0) + itemGap * (items.length - 1);
  let x = spec.width / 2 - total / 2;
  for (const it of items) {
    const c = spec.colors[it.color];
    el("rect", { x, y: legendY - sw / 2, width: sw, height: sw, rx: 5, fill: c.fill, stroke: c.stroke, "stroke-width": 1.5 });
    it.t.setAttribute("x", x + sw + swGap);
    x += it.w + itemGap;
  }

  return svg.outerHTML;
}, { spec, fontFaces, SANS, MONO });

fs.mkdirSync(OUT_DIR, { recursive: true });
const svgPath = path.join(OUT_DIR, SVG_NAME);
fs.writeFileSync(svgPath, svgMarkup);
console.log("wrote", svgPath, (svgMarkup.length / 1024).toFixed(1), "KB");

// rasterize the finished SVG through an <img>, exactly as the blog loads it
const svgDataUrl = "data:image/svg+xml;base64," + Buffer.from(svgMarkup).toString("base64");
const og = await browser.newPage({ viewport: { width: spec.width, height: spec.height }, deviceScaleFactor: 1 });
await og.setContent(`<html><body style="margin:0;background:${spec.bg}"><img id="i" src="${svgDataUrl}" width="${spec.width}" height="${spec.height}" style="display:block"></body></html>`);
await og.waitForFunction(() => document.getElementById("i").complete);
await og.waitForTimeout(300);
const pngPath = path.join(OUT_DIR, PNG_NAME);
await og.screenshot({ path: pngPath, clip: { x: 0, y: 0, width: spec.width, height: spec.height } });
console.log("wrote", pngPath);
await browser.close();
