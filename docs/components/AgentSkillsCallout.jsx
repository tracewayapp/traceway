import { useState } from "react";

const COMMAND = "npx skills add tracewayapp/traceway";

export default function AgentSkillsCallout() {
  const [copied, setCopied] = useState(false);

  async function copyCommand() {
    try {
      await navigator.clipboard.writeText(COMMAND);
      setCopied(true);
      setTimeout(() => setCopied(false), 2000);
    } catch {
      setCopied(false);
    }
  }

  return (
    <div className="agent-skills">
      <p className="agent-skills-text">
        Using a coding agent?{" "}
        <a
          href="https://tracewayapp.com/product/agent-skills"
          target="_blank"
          rel="noreferrer"
        >
          Install the Traceway skills
        </a>{" "}
        and run <code>/traceway-setup</code> to have it wire this up for you.
      </p>
      <button
        type="button"
        className="agent-skills-cmd"
        onClick={copyCommand}
        title={copied ? "Copied" : "Copy install command"}
      >
        <span className="agent-skills-prompt" aria-hidden>
          $
        </span>
        <span className="agent-skills-cmd-text">{COMMAND}</span>
        {copied ? (
          <svg
            className="agent-skills-cmd-icon agent-skills-cmd-icon--ok"
            width="15"
            height="15"
            viewBox="0 0 24 24"
            fill="none"
            stroke="currentColor"
            strokeWidth="2"
            strokeLinecap="round"
            strokeLinejoin="round"
            aria-hidden
          >
            <path d="M20 6 9 17l-5-5" />
          </svg>
        ) : (
          <svg
            className="agent-skills-cmd-icon"
            width="15"
            height="15"
            viewBox="0 0 24 24"
            fill="none"
            stroke="currentColor"
            strokeWidth="2"
            strokeLinecap="round"
            strokeLinejoin="round"
            aria-hidden
          >
            <rect width="14" height="14" x="8" y="8" rx="2" ry="2" />
            <path d="M4 16c-1.1 0-2-.9-2-2V4c0-1.1.9-2 2-2h10c1.1 0 2 .9 2 2" />
          </svg>
        )}
      </button>
    </div>
  );
}
