"use client";

import { useState } from "react";

const tabs = [
    {
        label: "Go",
        filename: "main.go",
        code: (
            <>
                <span className="text-purple-600 font-semibold">func</span> <span className="text-blue-600 font-semibold">main</span>() {"{"}{"\n"}
                {"  "}r := gin.<span className="text-blue-600">Default</span>(){"\n"}
                {"  "}r.<span className="text-blue-600">Use</span>(tracewaygin.<span className="text-blue-600">New</span>(<span className="text-green-600">{`"{TOKEN}@https://{SERVER_URL}/api/report"`}</span>)){"\n"}
                {"\n"}
                {"  "}r.<span className="text-blue-600">GET</span>(<span className="text-green-600">&quot;/test&quot;</span>, <span className="text-purple-600 font-semibold">func</span>(ctx *gin.Context) {"{"}{"\n"}
                {"    "}ctx.<span className="text-blue-600">AbortWithError</span>(<span className="text-orange-600">500</span>, fmt.<span className="text-blue-600">Errorf</span>(<span className="text-green-600">&quot;Worked!&quot;</span>)){"\n"}
                {"  "}{"}"}){"\n"}
                {"  "}r.<span className="text-blue-600">Run</span>(<span className="text-green-600">&quot;:8080&quot;</span>){"\n"}
                {"}"}
            </>
        ),
    },
    {
        label: "JavaScript",
        filename: "app.js",
        code: (
            <>
                <span className="text-purple-600 font-semibold">const</span> express = <span className="text-blue-600">require</span>(<span className="text-green-600">&quot;express&quot;</span>){"\n"}
                <span className="text-purple-600 font-semibold">const</span> traceway = <span className="text-blue-600">require</span>(<span className="text-green-600">&quot;@traceway/express&quot;</span>){"\n"}
                {"\n"}
                <span className="text-purple-600 font-semibold">const</span> app = <span className="text-blue-600">express</span>(){"\n"}
                app.<span className="text-blue-600">use</span>(traceway.<span className="text-blue-600">init</span>(<span className="text-green-600">{`"{TOKEN}@https://{SERVER_URL}/api/report"`}</span>)){"\n"}
                {"\n"}
                app.<span className="text-blue-600">get</span>(<span className="text-green-600">&quot;/test&quot;</span>, (req, res) =&gt; {"{"}{"\n"}
                {"  "}<span className="text-purple-600 font-semibold">throw new</span> <span className="text-blue-600">Error</span>(<span className="text-green-600">&quot;Worked!&quot;</span>){"\n"}
                {"}"}){"\n"}
                {"\n"}
                app.<span className="text-blue-600">listen</span>(<span className="text-orange-600">8080</span>)
            </>
        ),
    },
];

export function CodeTabs() {
    const [activeTab, setActiveTab] = useState(0);

    return (
        <div className="rounded-lg overflow-hidden bg-white border border-zinc-200 shadow-xl shadow-zinc-200/50">
            <div className="flex items-center justify-between px-3 py-2 bg-zinc-50 border-b border-zinc-100">
                <div className="flex items-center gap-1.5">
                    <div className="w-2.5 h-2.5 rounded-full bg-[#fe5f57]"></div>
                    <div className="w-2.5 h-2.5 rounded-full bg-[#fdbc2e]"></div>
                    <div className="w-2.5 h-2.5 rounded-full bg-[#28c841]"></div>
                </div>
                <div className="flex items-center gap-1">
                    {tabs.map((tab, i) => (
                        <button
                            key={tab.label}
                            onClick={() => setActiveTab(i)}
                            className={`px-2.5 py-0.5 rounded text-[10px] font-mono font-medium transition-colors ${
                                activeTab === i
                                    ? "bg-white text-zinc-900 shadow-sm border border-zinc-200"
                                    : "text-zinc-400 hover:text-zinc-600"
                            }`}
                        >
                            {tab.filename}
                        </button>
                    ))}
                </div>
            </div>
            <div className="p-0 overflow-x-auto bg-white">
                <pre className="p-4 text-xs font-mono leading-relaxed text-zinc-800">
                    <code className="block">
                        {tabs[activeTab].code}
                    </code>
                </pre>
            </div>
        </div>
    );
}
