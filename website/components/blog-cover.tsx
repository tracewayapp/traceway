import type { ReactNode } from "react";

const W = 1200;
const H = 520;

const MOTIFS = ["waterfall", "bars", "rings", "heatmap", "wave"] as const;
type Motif = (typeof MOTIFS)[number];

type Palette = { a: string; b: string; c: string; hot: string };

const PALETTES: Palette[] = [
  { a: "#4f8cff", b: "#22d3ee", c: "#8b7bff", hot: "#ff4d8d" },
  { a: "#8b7bff", b: "#4f8cff", c: "#22d3ee", hot: "#ffd166" },
  { a: "#22d3ee", b: "#22e0a8", c: "#4f8cff", hot: "#ff4d8d" },
  { a: "#4f8cff", b: "#8b7bff", c: "#22e0a8", hot: "#ffd166" },
];

function hash(text: string): number {
  let h = 2166136261;
  for (let i = 0; i < text.length; i++) {
    h ^= text.charCodeAt(i);
    h = Math.imul(h, 16777619);
  }
  return h >>> 0;
}

function prng(seed: number): () => number {
  let s = seed;
  return () => {
    s = (s + 0x6d2b79f5) | 0;
    let t = Math.imul(s ^ (s >>> 15), 1 | s);
    t = (t + Math.imul(t ^ (t >>> 7), 61 | t)) ^ t;
    return ((t ^ (t >>> 14)) >>> 0) / 4294967296;
  };
}

const n = (value: number) => Math.round(value * 10) / 10;

function waterfall(rnd: () => number, p: Palette, uid: string): ReactNode {
  type Bar = { x: number; w: number; depth: number };
  const bars: Bar[] = [{ x: 90, w: 1020, depth: 0 }];
  let cursor = 120;
  while (bars.length < 12) {
    const w = 130 + rnd() * 280;
    bars.push({ x: cursor, w, depth: 1 });
    const children = Math.floor(rnd() * 3);
    let inner = cursor + 16;
    for (let j = 0; j < children && bars.length < 12; j++) {
      const cw = (w - 32) * (0.25 + rnd() * 0.4);
      bars.push({ x: inner, w: cw, depth: 2 });
      inner += cw * (0.6 + rnd() * 0.5);
    }
    cursor += w * (0.5 + rnd() * 0.45);
    if (cursor > 860) cursor = 160 + rnd() * 240;
  }
  const hot = 3 + Math.floor(rnd() * (bars.length - 4));
  const colors = [p.a, p.b, p.c];
  const rowH = 40;
  const top = 34;
  return (
    <g transform={`rotate(-7 ${W / 2} ${H / 2}) scale(1.08) translate(-40 -14)`}>
      {[0, 1, 2, 3, 4, 5].map((i) => (
        <line
          key={`grid-${i}`}
          x1={90 + i * 204}
          x2={90 + i * 204}
          y1={-40}
          y2={H + 60}
          stroke="#fff"
          strokeOpacity={0.05}
          strokeDasharray="2 8"
        />
      ))}
      {bars.map((bar, i) => {
        const y = top + i * rowH;
        const width = Math.min(bar.w, 1110 - bar.x);
        const color = i === hot ? p.hot : colors[bar.depth % colors.length];
        return (
          <g key={i}>
            <line
              x1={90}
              x2={1110}
              y1={y + 11}
              y2={y + 11}
              stroke="#fff"
              strokeOpacity={0.035}
            />
            <rect
              x={n(bar.x)}
              y={y}
              width={n(width)}
              height={22}
              rx={7}
              fill={color}
              opacity={0.22}
              filter={`url(#${uid}-glow)`}
            />
            <rect
              x={n(bar.x)}
              y={y}
              width={n(width)}
              height={22}
              rx={7}
              fill={color}
              opacity={bar.depth === 0 ? 0.55 : 0.92}
            />
            <rect
              x={n(bar.x + 8)}
              y={y + 5}
              width={n(Math.min(width * 0.35, 90))}
              height={3}
              rx={1.5}
              fill="#fff"
              opacity={0.35}
            />
          </g>
        );
      })}
    </g>
  );
}

function barsMotif(rnd: () => number, p: Palette, uid: string): ReactNode {
  const count = 46;
  const pitch = 23;
  const left = (W - count * pitch) / 2;
  const base = 452;
  const mu = 0.1 + rnd() * 0.25;
  const sigma = 0.5 + rnd() * 0.15;
  const raw = Array.from({ length: count }, (_, i) => {
    const t = ((i + 1) / count) * 5;
    const pdf = Math.exp(-((Math.log(t) - mu) ** 2) / (2 * sigma * sigma)) / t;
    return pdf * (0.82 + rnd() * 0.36);
  });
  const peak = Math.max(...raw);
  const tail = Math.round(count * (0.62 + rnd() * 0.08));
  const markX = left + tail * pitch - 4;
  return (
    <g>
      {[0, 1, 2, 3].map((i) => (
        <line
          key={i}
          x1={60}
          x2={W - 60}
          y1={base - 96 * (i + 1)}
          y2={base - 96 * (i + 1)}
          stroke="#fff"
          strokeOpacity={0.05}
          strokeDasharray="2 8"
        />
      ))}
      {raw.map((value, i) => {
        const h = Math.max(6, (value / peak) * 360);
        const color = i >= tail ? p.hot : `url(#${uid}-bar)`;
        return (
          <rect
            key={i}
            x={n(left + i * pitch)}
            y={n(base - h)}
            width={15}
            height={n(h)}
            rx={5}
            fill={color}
            opacity={i >= tail ? 0.9 : 0.95}
          />
        );
      })}
      <line
        x1={markX}
        x2={markX}
        y1={70}
        y2={base + 14}
        stroke={p.hot}
        strokeOpacity={0.75}
        strokeWidth={1.5}
        strokeDasharray="5 6"
      />
      <circle cx={markX} cy={70} r={5} fill={p.hot} />
      <line
        x1={60}
        x2={W - 60}
        y1={base + 14}
        y2={base + 14}
        stroke="#fff"
        strokeOpacity={0.14}
      />
    </g>
  );
}

function rings(rnd: () => number, p: Palette, uid: string): ReactNode {
  const cx = 700 + rnd() * 240;
  const cy = 200 + rnd() * 120;
  const colors = [p.a, p.b, p.c];
  const items: ReactNode[] = [];
  for (let i = 0; i < 9; i++) {
    const r = 54 + i * 62;
    const circumference = 2 * Math.PI * r;
    const segments = 3 + Math.floor(rnd() * 5);
    const fill = 0.45 + rnd() * 0.35;
    const dash = (circumference / segments) * fill;
    const gap = circumference / segments - dash;
    const rotation = rnd() * 360;
    const color = colors[i % colors.length];
    const thin = i % 3 === 2;
    items.push(
      <circle
        key={`ring-${i}`}
        cx={n(cx)}
        cy={n(cy)}
        r={r}
        fill="none"
        stroke={color}
        strokeWidth={thin ? 1.5 : 9}
        strokeLinecap="round"
        strokeDasharray={thin ? "2 10" : `${n(dash)} ${n(gap)}`}
        strokeOpacity={thin ? 0.5 : Math.max(0.18, 0.9 - i * 0.085)}
        transform={`rotate(${n(rotation)} ${n(cx)} ${n(cy)})`}
      />,
    );
    if (!thin && rnd() > 0.35) {
      const angle = rnd() * Math.PI * 2;
      const hot = rnd() > 0.6;
      const x = cx + Math.cos(angle) * r;
      const y = cy + Math.sin(angle) * r;
      items.push(
        <g key={`node-${i}`}>
          <circle
            cx={n(x)}
            cy={n(y)}
            r={22}
            fill={hot ? p.hot : "#fff"}
            opacity={0.25}
            filter={`url(#${uid}-glow)`}
          />
          <circle cx={n(x)} cy={n(y)} r={hot ? 10 : 7} fill={hot ? p.hot : "#fff"} />
        </g>,
      );
    }
  }
  return <g>{items}</g>;
}

function heatmap(rnd: () => number, p: Palette): ReactNode {
  const pitch = 34;
  const cols = Math.ceil(W / pitch) + 1;
  const rows = Math.ceil(H / pitch) + 1;
  const phases = Array.from({ length: 6 }, () => rnd() * Math.PI * 2);
  const fx = 0.12 + rnd() * 0.1;
  const fy = 0.16 + rnd() * 0.12;
  const values: number[] = [];
  for (let row = 0; row < rows; row++) {
    for (let col = 0; col < cols; col++) {
      const field =
        Math.sin(col * fx + phases[0]) +
        Math.sin(row * fy + phases[1]) +
        Math.sin((col + row) * 0.11 + phases[2]) +
        Math.sin((col - row) * 0.17 + phases[3]);
      values.push(field + (rnd() - 0.5) * 1.3);
    }
  }
  const low = Math.min(...values);
  const span = Math.max(...values) - low || 1;
  const cells: ReactNode[] = values.map((raw, i) => {
    const value = (raw - low) / span;
    const hot = value > 0.8 && rnd() > 0.5;
    const color = hot ? p.hot : value > 0.55 ? p.b : p.a;
    return (
      <rect
        key={i}
        x={(i % cols) * pitch - 6}
        y={Math.floor(i / cols) * pitch - 8}
        width={pitch - 7}
        height={pitch - 7}
        rx={6}
        fill={color}
        opacity={n(hot ? 0.95 : 0.05 + value ** 2.6 * 0.95)}
      />
    );
  });
  return <g>{cells}</g>;
}

function wave(rnd: () => number, p: Palette, uid: string): ReactNode {
  const points = 13;
  const step = (W + 80) / (points - 1);
  const series = [p.c, p.b, p.a].map((color, index) => {
    let y = 150 + index * 95 + rnd() * 40;
    const ys = Array.from({ length: points }, () => {
      y += (rnd() - 0.5) * 110;
      y = Math.min(430, Math.max(70 + index * 60, y));
      return y;
    });
    return { color, ys };
  });
  const path = (ys: number[]) => {
    let d = `M ${-40} ${n(ys[0])}`;
    for (let i = 1; i < ys.length; i++) {
      const x0 = -40 + (i - 1) * step;
      const x1 = -40 + i * step;
      const mid = (x0 + x1) / 2;
      d += ` C ${n(mid)} ${n(ys[i - 1])}, ${n(mid)} ${n(ys[i])}, ${n(x1)} ${n(ys[i])}`;
    }
    return d;
  };
  const markIndex = 4 + Math.floor(rnd() * 5);
  const markX = -40 + markIndex * step;
  const front = series[series.length - 1];
  return (
    <g>
      {[0, 1, 2, 3].map((i) => (
        <line
          key={i}
          x1={0}
          x2={W}
          y1={110 + i * 100}
          y2={110 + i * 100}
          stroke="#fff"
          strokeOpacity={0.05}
          strokeDasharray="2 8"
        />
      ))}
      {series.map(({ color, ys }, index) => {
        const d = path(ys);
        const last = index === series.length - 1;
        return (
          <g key={index}>
            <path
              d={`${d} L ${W + 40} ${H + 20} L -40 ${H + 20} Z`}
              fill={color}
              opacity={last ? 0.26 : 0.13}
            />
            <path
              d={d}
              fill="none"
              stroke={color}
              strokeWidth={last ? 12 : 8}
              opacity={last ? 0.45 : 0.3}
              filter={`url(#${uid}-glow)`}
            />
            <path
              d={d}
              fill="none"
              stroke={color}
              strokeWidth={last ? 4 : 2.5}
              strokeOpacity={last ? 1 : 0.85}
              strokeLinecap="round"
            />
          </g>
        );
      })}
      <line
        x1={n(markX)}
        x2={n(markX)}
        y1={40}
        y2={H - 30}
        stroke="#fff"
        strokeOpacity={0.3}
        strokeDasharray="4 7"
      />
      <circle cx={n(markX)} cy={n(front.ys[markIndex])} r={20} fill={p.hot} opacity={0.3} filter={`url(#${uid}-glow)`} />
      <circle cx={n(markX)} cy={n(front.ys[markIndex])} r={8} fill={p.hot} stroke="#0a0d1a" strokeWidth={3} />
    </g>
  );
}

/**
 * Cover art for a post. `cover` in the front matter is either a path to an
 * image or the name of a motif; with neither, the motif and palette follow
 * from the slug, so a post keeps the same cover everywhere it is shown.
 */
export function BlogCover({
  slug,
  cover,
  className,
}: {
  slug: string;
  cover?: string;
  className?: string;
}) {
  if (cover?.startsWith("/")) {
    return (
      <div className={`blog-cover ${className ?? ""}`}>
        {/* eslint-disable-next-line @next/next/no-img-element */}
        <img src={cover} alt="" />
      </div>
    );
  }

  const seed = hash(slug);
  const rnd = prng(seed);
  const motif: Motif = MOTIFS.includes(cover as Motif)
    ? (cover as Motif)
    : MOTIFS[seed % MOTIFS.length];
  const palette = PALETTES[(seed >>> 8) % PALETTES.length];
  const uid = `cover-${seed.toString(36)}`;
  const glowX = 15 + rnd() * 30;

  let art: ReactNode;
  switch (motif) {
    case "waterfall":
      art = waterfall(rnd, palette, uid);
      break;
    case "bars":
      art = barsMotif(rnd, palette, uid);
      break;
    case "rings":
      art = rings(rnd, palette, uid);
      break;
    case "heatmap":
      art = heatmap(rnd, palette);
      break;
    default:
      art = wave(rnd, palette, uid);
  }

  return (
    <div className={`blog-cover ${className ?? ""}`}>
      <svg
        viewBox={`0 0 ${W} ${H}`}
        preserveAspectRatio="xMidYMid slice"
        aria-hidden="true"
        focusable="false"
      >
        <defs>
          <linearGradient id={`${uid}-bg`} x1="0" y1="0" x2="1" y2="1">
            <stop offset="0" stopColor="#111a36" />
            <stop offset="1" stopColor="#080a15" />
          </linearGradient>
          <radialGradient id={`${uid}-halo`} cx={`${n(glowX)}%`} cy="0%" r="75%">
            <stop offset="0" stopColor={palette.a} stopOpacity="0.38" />
            <stop offset="1" stopColor={palette.a} stopOpacity="0" />
          </radialGradient>
          <radialGradient id={`${uid}-halo2`} cx="95%" cy="100%" r="60%">
            <stop offset="0" stopColor={palette.c} stopOpacity="0.28" />
            <stop offset="1" stopColor={palette.c} stopOpacity="0" />
          </radialGradient>
          <radialGradient id={`${uid}-vignette`} cx="50%" cy="50%" r="72%">
            <stop offset="0.55" stopColor="#05070c" stopOpacity="0" />
            <stop offset="1" stopColor="#05070c" stopOpacity="0.7" />
          </radialGradient>
          <linearGradient id={`${uid}-bar`} x1="0" y1="0" x2="0" y2="1">
            <stop offset="0" stopColor={palette.b} />
            <stop offset="1" stopColor={palette.a} />
          </linearGradient>
          <pattern id={`${uid}-dots`} width="24" height="24" patternUnits="userSpaceOnUse">
            <circle cx="2" cy="2" r="1.1" fill="#fff" opacity="0.1" />
          </pattern>
          <filter id={`${uid}-glow`} x="-50%" y="-200%" width="200%" height="500%">
            <feGaussianBlur stdDeviation="12" />
          </filter>
        </defs>
        <rect width={W} height={H} fill={`url(#${uid}-bg)`} />
        <rect width={W} height={H} fill={`url(#${uid}-dots)`} />
        <rect width={W} height={H} fill={`url(#${uid}-halo)`} />
        <rect width={W} height={H} fill={`url(#${uid}-halo2)`} />
        {art}
        <rect width={W} height={H} fill={`url(#${uid}-vignette)`} />
      </svg>
    </div>
  );
}
