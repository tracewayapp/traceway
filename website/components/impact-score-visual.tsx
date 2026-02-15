export function ImpactScoreVisual() {
  const endpoints = [
    {
      method: "POST",
      path: "/api/checkout",
      score: "Critical",
      color: "bg-red-100 text-red-700",
      p50: "320ms",
      p95: "1.8s",
      p99: "4.2s",
      requests: "12.4k",
    },
    {
      method: "GET",
      path: "/api/users",
      score: "High",
      color: "bg-orange-100 text-orange-700",
      p50: "85ms",
      p95: "420ms",
      p99: "1.1s",
      requests: "48.2k",
    },
    {
      method: "POST",
      path: "/api/upload",
      score: "Medium",
      color: "bg-yellow-100 text-yellow-700",
      p50: "210ms",
      p95: "680ms",
      p99: "950ms",
      requests: "3.1k",
    },
    {
      method: "GET",
      path: "/api/health",
      score: "Good",
      color: "bg-green-100 text-green-700",
      p50: "2ms",
      p95: "8ms",
      p99: "15ms",
      requests: "102k",
    },
  ];

  const slis = [
    "Inverted apdex variant",
    "Error rate floor",
    "P99 floor",
    "Client error floor",
    "Volume error floor",
  ];

  return (
    <div className="w-full">
      <div className="rounded-xl border border-zinc-200 bg-white overflow-hidden">
        {/* Window chrome */}
        <div className="flex items-center gap-1.5 px-4 py-3 bg-zinc-50 border-b border-zinc-100">
          <div className="w-2.5 h-2.5 rounded-full bg-[#fe5f57]"></div>
          <div className="w-2.5 h-2.5 rounded-full bg-[#fdbc2e]"></div>
          <div className="w-2.5 h-2.5 rounded-full bg-[#28c841]"></div>
          <span className="ml-3 text-[11px] font-medium text-zinc-400">
            Endpoints - Impact Score
          </span>
        </div>

        {/* Table header */}
        <div className="grid grid-cols-[1fr_90px_80px_80px_80px_70px] gap-2 px-4 py-2.5 bg-zinc-50/50 border-b border-zinc-100 text-[11px] font-semibold text-zinc-400 uppercase tracking-wider">
          <div>Endpoint</div>
          <div>Score</div>
          <div className="text-right">P50</div>
          <div className="text-right">P95</div>
          <div className="text-right">P99</div>
          <div className="text-right">Reqs</div>
        </div>

        {/* Table rows */}
        {endpoints.map((ep) => (
          <div
            key={ep.path}
            className="grid grid-cols-[1fr_90px_80px_80px_80px_70px] gap-2 px-4 py-3 border-b border-zinc-50 last:border-b-0 hover:bg-zinc-50/50 transition-colors"
          >
            <div className="font-mono text-sm text-zinc-800 truncate">
              <span className="text-zinc-400 font-medium">{ep.method}</span>{" "}
              {ep.path}
            </div>
            <div>
              <span
                className={`inline-flex items-center px-2 py-0.5 rounded-full text-[11px] font-semibold ${ep.color}`}
              >
                {ep.score}
              </span>
            </div>
            <div className="text-right text-sm text-zinc-600 font-mono">
              {ep.p50}
            </div>
            <div className="text-right text-sm text-zinc-600 font-mono">
              {ep.p95}
            </div>
            <div className="text-right text-sm text-zinc-600 font-mono">
              {ep.p99}
            </div>
            <div className="text-right text-sm text-zinc-500 font-mono">
              {ep.requests}
            </div>
          </div>
        ))}
      </div>

      {/* SLI note */}
      <p className="mt-4 text-center text-sm text-zinc-500">
        The Impact Score takes the{" "}
        <span className="font-medium text-zinc-700">max</span> across five SLIs:{" "}
        {slis.map((sli, i) => (
          <span key={sli}>
            <span className="font-medium text-zinc-600">{sli}</span>
            {i < slis.length - 1 ? ", " : "."}
          </span>
        ))}
      </p>
    </div>
  );
}
