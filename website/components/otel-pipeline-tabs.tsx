"use client";

import { useState } from "react";

const TABS = [
  {
    filename: "builder-config.yaml",
    code: (
      <>
        <span style={{ color: "var(--fg-3)" }}>
          # compile the processor into your collector (ocb manifest)
        </span>
        {"\n"}
        <span style={{ color: "var(--fg-1)" }}>processors:</span>
        {"\n"}
        {"  - "}
        <span style={{ color: "var(--a2)" }}>gomod:</span>{" "}
        <span style={{ color: "var(--ok)" }}>
          github.com/tracewayapp/traceway/backend v1.8.0
        </span>
        {"\n"}
        {"    "}
        <span style={{ color: "var(--a2)" }}>import:</span>{" "}
        <span style={{ color: "var(--ok)" }}>
          github.com/tracewayapp/traceway/backend/app/symbolicator/otelprocessor
        </span>
        {"\n\n"}
        <span style={{ color: "var(--a2)" }}>$</span>{" "}
        <span style={{ color: "var(--fg-1)" }}>
          ocb --config builder-config.yaml
        </span>
      </>
    ),
  },
  {
    filename: "otel-collector.yaml",
    code: (
      <>
        <span style={{ color: "var(--fg-3)" }}>
          # then reference it in the collector config
        </span>
        {"\n"}
        <span style={{ color: "var(--fg-1)" }}>processors:</span>
        {"\n"}
        {"  "}
        <span style={{ color: "var(--a2)" }}>source_map_symbolicator:</span>
        {"\n"}
        <span style={{ color: "var(--fg-3)" }}>
          {"    source_map_store: file_store\n"}
          {"    local_source_maps:\n"}
          {"      path: /sourcemaps\n"}
          {"    cache_dir: /var/cache/symbolicator\n"}
          {"    cache_max_disk_pct: 50"}
        </span>
        {"\n\n"}
        <span style={{ color: "var(--fg-1)" }}>
          {"service:\n"}
          {"  pipelines:\n"}
          {"    traces:"}
        </span>
        {"\n"}
        <span style={{ color: "var(--ok)" }}>
          {"      processors: [source_map_symbolicator]"}
        </span>
      </>
    ),
  },
];

export function OtelPipelineTabs() {
  const [activeTab, setActiveTab] = useState(0);

  return (
    <div className="term">
      <div className="term-head">
        <span className="tdot" style={{ background: "#ff5a5f" }} />
        <span className="tdot" style={{ background: "#ffd166" }} />
        <span className="tdot" style={{ background: "#22e0a8" }} />
        <div className="ml-auto flex items-center gap-1">
          {TABS.map((tab, i) => (
            <button
              key={tab.filename}
              onClick={() => setActiveTab(i)}
              className="rounded px-2.5 py-0.5 text-[10.5px] transition-colors"
              style={{
                fontFamily: "var(--font-mono)",
                color: activeTab === i ? "var(--fg-0)" : "var(--fg-3)",
                background:
                  activeTab === i ? "rgba(255, 255, 255, 0.08)" : "transparent",
                border:
                  activeTab === i
                    ? "1px solid var(--hair-2)"
                    : "1px solid transparent",
              }}
            >
              {tab.filename}
            </button>
          ))}
        </div>
      </div>
      <div className="overflow-x-auto">
        <pre
          className="px-[22px] py-4 text-[13px] leading-[1.75]"
          style={{ fontFamily: "var(--font-mono)", margin: 0 }}
        >
          {TABS[activeTab].code}
        </pre>
      </div>
    </div>
  );
}
