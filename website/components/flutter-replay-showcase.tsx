"use client";

import { useEffect, useRef, useState } from "react";
import { Archive, Play, Pause, Maximize } from "lucide-react";

import "rrweb-player/dist/style.css";

type EventT = { timestamp: number } & Record<string, unknown>;

interface PlayerLike {
  play: () => void;
  pause: () => void;
  goto: (offsetMs: number) => void;
  setSpeed: (speed: number) => void;
  toggleSkipInactive: () => void;
  addEventListener: (name: string, cb: (...args: unknown[]) => void) => void;
  getMetaData: () => { totalTime: number; startTime: number; endTime: number };
  getReplayer: () => { getCurrentTime: () => number };
  $destroy?: () => void;
}

const SPEEDS = [1, 2, 4, 8] as const;
type Speed = (typeof SPEEDS)[number];

const PHONE_WIDTH = 260;
const PHONE_PADDING = 8;
const SCREEN_WIDTH = PHONE_WIDTH - PHONE_PADDING * 2; // 244
// Recording was captured at 323.84 × 576 (≈ 9:16). Phone screen keeps that
// aspect so the content fills edge-to-edge without letterboxing.
const RECORDING_W = 323.84;
const RECORDING_H = 576;
const SCREEN_HEIGHT = Math.round((SCREEN_WIDTH * RECORDING_H) / RECORDING_W);

export function FlutterReplayShowcase({
  eventsUrl = "/events.json",
}: {
  eventsUrl?: string;
}) {
  const [events, setEvents] = useState<EventT[] | null>(null);
  const [loadError, setLoadError] = useState<string | null>(null);
  const [isPlaying, setIsPlaying] = useState(false);
  const [currentMs, setCurrentMs] = useState(0);
  const [durationMs, setDurationMs] = useState(0);
  const [speed, setSpeedState] = useState<Speed>(1);

  const screenRef = useRef<HTMLDivElement | null>(null);
  const playerRef = useRef<PlayerLike | null>(null);
  const phoneWrapperRef = useRef<HTMLDivElement | null>(null);

  // Fetch events
  useEffect(() => {
    let cancelled = false;
    (async () => {
      try {
        const res = await fetch(eventsUrl);
        if (!res.ok) throw new Error(`HTTP ${res.status}`);
        const json = (await res.json()) as EventT[];
        if (!cancelled) setEvents(json);
      } catch (err) {
        if (!cancelled)
          setLoadError(err instanceof Error ? err.message : String(err));
      }
    })();
    return () => {
      cancelled = true;
    };
  }, [eventsUrl]);

  // Mount rrweb-player once events are loaded
  useEffect(() => {
    if (!events || events.length < 2 || !screenRef.current) return;

    let cancelled = false;
    let tick: ReturnType<typeof setInterval> | null = null;
    let player: PlayerLike | null = null;

    (async () => {
      const mod = await import("rrweb-player");
      if (cancelled || !screenRef.current) return;

      const PlayerCtor = mod.default as unknown as new (opts: {
        target: HTMLElement;
        props: {
          events: EventT[];
          autoPlay?: boolean;
          showController?: boolean;
          skipInactive?: boolean;
          mouseTail?: boolean;
          width?: number;
          height?: number;
        };
      }) => PlayerLike;

      screenRef.current.innerHTML = "";
      player = new PlayerCtor({
        target: screenRef.current,
        props: {
          events,
          autoPlay: false,
          showController: false,
          skipInactive: true,
          mouseTail: false,
          width: SCREEN_WIDTH,
          height: SCREEN_HEIGHT,
        },
      });
      playerRef.current = player;

      const meta = player.getMetaData();
      setDurationMs(meta.totalTime);

      // Keep UI state in sync with rrweb-player's real lifecycle
      player.addEventListener("finish", () => {
        setIsPlaying(false);
        if (playerRef.current) {
          const r = playerRef.current.getReplayer();
          if (r) setCurrentMs(r.getCurrentTime());
        }
      });
      player.addEventListener("pause", () => setIsPlaying(false));
      player.addEventListener("resume", () => setIsPlaying(true));
      player.addEventListener("start", () => setIsPlaying(true));

      // Poll the replayer's clock for the progress bar
      tick = setInterval(() => {
        const p = playerRef.current;
        if (!p) return;
        const r = p.getReplayer();
        if (r) setCurrentMs(r.getCurrentTime());
      }, 100);
    })();

    return () => {
      cancelled = true;
      if (tick) clearInterval(tick);
      if (player?.$destroy) player.$destroy();
      playerRef.current = null;
    };
  }, [events]);

  // ── control handlers ─────────────────────────────────────────────────────
  const togglePlay = () => {
    const p = playerRef.current;
    if (!p) return;
    if (isPlaying) {
      p.pause();
      return;
    }
    // If we're at the end, restart from zero first.
    if (durationMs > 0 && currentMs >= durationMs - 100) {
      p.goto(0);
    }
    p.play();
  };

  const changeSpeed = (s: Speed) => {
    const p = playerRef.current;
    if (!p) return;
    p.setSpeed(s);
    setSpeedState(s);
  };

  const seek = (ms: number) => {
    const p = playerRef.current;
    if (!p) return;
    const clamped = Math.max(0, Math.min(durationMs, ms));
    p.goto(clamped);
    setCurrentMs(clamped);
  };

  const fullscreen = () => {
    const el = phoneWrapperRef.current;
    if (!el) return;
    if (document.fullscreenElement) document.exitFullscreen();
    else el.requestFullscreen?.();
  };

  const onProgressClick = (e: React.MouseEvent<HTMLDivElement>) => {
    const rect = e.currentTarget.getBoundingClientRect();
    const ratio = (e.clientX - rect.left) / rect.width;
    seek(ratio * durationMs);
  };

  const percent = durationMs > 0 ? (currentMs / durationMs) * 100 : 0;

  return (
    <section className="wrap pb-24">
      <div className="max-w-5xl mx-auto">
        <div
          className="rounded-[14px] overflow-hidden"
          style={{
            background: "var(--ink-0)",
            border: "1px solid var(--hair-2)",
          }}
        >
          {/* Issue header */}
          <div
            className="flex flex-wrap items-center justify-between gap-3 px-5 sm:px-7 pt-5 pb-4"
            style={{
              background: "var(--ink-1)",
              borderBottom: "1px solid var(--hair)",
            }}
          >
            <div className="min-w-0">
              <h4
                className="text-[15px]"
                style={{
                  fontFamily: "var(--font-display)",
                  color: "var(--fg-0)",
                  letterSpacing: "-0.005em",
                }}
              >
                Stack Trace
              </h4>
              <p
                className="mt-1 text-[11.5px] leading-relaxed"
                style={{
                  color: "var(--fg-3)",
                  fontFamily: "var(--font-mono)",
                }}
              >
                First seen: Apr 6, 2026, 10:17 AM · Last seen: Apr 10, 2026,
                1:53 AM · Total occurrences: 23 · Platform: Flutter 3.27.1
              </p>
            </div>
            <button
              type="button"
              className="inline-flex items-center gap-1.5 text-[12px] px-2.5 py-1 rounded-md"
              style={{
                border: "1px solid var(--hair-2)",
                color: "var(--fg-1)",
                background:
                  "color-mix(in oklab, var(--ink-2) 60%, transparent)",
                fontFamily: "var(--font-display)",
              }}
            >
              <Archive className="h-3.5 w-3.5" />
              Archive
            </button>
          </div>

          {/* Issue body: trace text + embedded replay */}
          <div className="grid gap-8 md:grid-cols-[minmax(0,1.4fr)_minmax(280px,0.8fr)] items-start px-5 sm:px-7 py-6">
            <div
              className="text-[11px] sm:text-[12px] leading-[1.75] min-w-0"
              style={{
                fontFamily: "var(--font-mono)",
                overflowWrap: "anywhere",
                wordBreak: "break-word",
                color: "var(--fg-1)",
              }}
            >
              <div style={{ color: "var(--crit)" }}>
                _TypeError: type &apos;Null&apos; is not a subtype of type
                &apos;String&apos; in type cast
              </div>
              <div className="mt-3" style={{ color: "var(--fg-3)" }}>
                ▸ 12 framework frames (package:flutter)
              </div>
              <StackFrame
                name="_ChatScreenState._onSendPressed"
                file="package:assistant_app/screens/chat_screen.dart:121:18"
              />
              <StackFrame
                name="ConversationRepository.sendMessage"
                file="package:assistant_app/data/conversation_repository.dart:64:24"
              />
              <StackFrame
                name="StreamedResponse.parseChunk"
                file="package:assistant_app/data/streamed_response.dart:41:12"
              />
              <StackFrame
                name="ApiClient.streamCompletion"
                file="package:assistant_app/data/api_client.dart:90:20"
              />
              <div className="mt-3" style={{ color: "var(--fg-3)" }}>
                ▸ 4 async gap frames
              </div>
            </div>

            {/* Embedded replay: phone + control bar */}
            <div className="flex flex-col items-center min-w-0 w-full justify-self-center">
                <div
                  ref={phoneWrapperRef}
                  className="flex flex-col items-center gap-4"
                >
                  {/* Phone */}
                  <div
                    className="relative"
                    style={{
                      width: PHONE_WIDTH,
                      aspectRatio: `${RECORDING_W} / ${RECORDING_H}`,
                      borderRadius: 36,
                      padding: PHONE_PADDING,
                      background: "var(--ink-2)",
                      border: "1px solid var(--hair-2)",
                      boxShadow: "0 32px 48px -32px rgba(0, 0, 0, 0.6)",
                    }}
                  >
                    <div
                      className="relative w-full h-full overflow-hidden"
                      style={{
                        borderRadius: 30,
                        background: "#1f1f1f",
                      }}
                    >
                      {loadError ? (
                        <Status text={`Failed: ${loadError}`} />
                      ) : !events ? (
                        <Status text="Loading replay…" />
                      ) : (
                        <div
                          ref={screenRef}
                          className="flutter-replay-mount absolute inset-0"
                          style={{ overflow: "hidden" }}
                        />
                      )}
                    </div>
                  </div>

                  {/* Control bar: external, styled to match Traceway dashboard */}
                  <div
                    className="w-full rounded-xl px-4 py-3"
                    style={{
                      background:
                        "color-mix(in oklab, var(--ink-0) 70%, transparent)",
                      border: "1px solid var(--hair)",
                      minWidth: PHONE_WIDTH,
                      maxWidth: PHONE_WIDTH + 40,
                    }}
                  >
                    {/* Progress row */}
                    <div
                      className="flex items-center gap-3 text-[11px]"
                      style={{
                        fontFamily: "var(--font-mono)",
                        color: "var(--fg-3)",
                      }}
                    >
                      <span>{formatMs(currentMs)}</span>
                      <div
                        className="relative flex-1 cursor-pointer"
                        onClick={onProgressClick}
                        style={{
                          height: 6,
                          borderRadius: 3,
                          background: "var(--ink-3)",
                        }}
                      >
                        <div
                          style={{
                            width: `${percent}%`,
                            height: "100%",
                            borderRadius: 3,
                            background: "var(--a1)",
                          }}
                        />
                        <div
                          style={{
                            position: "absolute",
                            left: `${percent}%`,
                            top: "50%",
                            transform: "translate(-50%, -50%)",
                            width: 12,
                            height: 12,
                            borderRadius: "50%",
                            background: "var(--a1)",
                            boxShadow:
                              "0 0 0 3px color-mix(in oklab, var(--a1) 20%, transparent)",
                          }}
                        />
                      </div>
                      <span>{formatMs(durationMs)}</span>
                    </div>

                    {/* Buttons row */}
                    <div
                      className="mt-3 flex items-center justify-center gap-2 text-[12px]"
                      style={{ fontFamily: "var(--font-mono)" }}
                    >
                      <IconButton
                        onClick={togglePlay}
                        label={isPlaying ? "Pause" : "Play"}
                      >
                        {isPlaying ? (
                          <Pause
                            className="h-3.5 w-3.5"
                            fill="currentColor"
                            strokeWidth={0}
                          />
                        ) : (
                          <Play
                            className="h-3.5 w-3.5 ml-0.5"
                            fill="currentColor"
                            strokeWidth={0}
                          />
                        )}
                      </IconButton>
                      {SPEEDS.map((s) => (
                        <SpeedChip
                          key={s}
                          active={speed === s}
                          onClick={() => changeSpeed(s)}
                        >
                          {s}x
                        </SpeedChip>
                      ))}
                      <IconButton onClick={fullscreen} label="Fullscreen">
                        <Maximize className="h-3.5 w-3.5" />
                      </IconButton>
                    </div>
                  </div>
                </div>
            </div>
          </div>
        </div>
      </div>
    </section>
  );
}

function StackFrame({ name, file }: { name: string; file: string }) {
  return (
    <div className="mt-2">
      <div style={{ color: "var(--fg-0)" }}>{name}</div>
      <div style={{ color: "var(--fg-3)" }}>{file}</div>
    </div>
  );
}

function Status({ text }: { text: string }) {
  return (
    <div
      className="absolute inset-0 grid place-items-center px-6 text-center text-[11px]"
      style={{
        color: "var(--fg-3)",
        fontFamily: "var(--font-mono)",
        letterSpacing: "0.04em",
      }}
    >
      {text}
    </div>
  );
}

function IconButton({
  children,
  onClick,
  label,
}: {
  children: React.ReactNode;
  onClick: () => void;
  label: string;
}) {
  return (
    <button
      type="button"
      onClick={onClick}
      aria-label={label}
      className="grid place-items-center rounded-full transition-colors"
      style={{
        width: 28,
        height: 28,
        color: "var(--fg-0)",
        background: "transparent",
      }}
    >
      {children}
    </button>
  );
}

function SpeedChip({
  children,
  active,
  onClick,
}: {
  children: React.ReactNode;
  active: boolean;
  onClick: () => void;
}) {
  return (
    <button
      type="button"
      onClick={onClick}
      className="rounded-full transition-colors"
      style={{
        minWidth: 28,
        height: 24,
        padding: "0 9px",
        background: active ? "var(--a1)" : "transparent",
        color: active ? "#0a0612" : "var(--fg-2)",
        fontWeight: active ? 600 : 400,
      }}
    >
      {children}
    </button>
  );
}

function formatMs(ms: number): string {
  const t = Math.max(0, Math.floor(ms / 1000));
  const m = Math.floor(t / 60);
  const s = t % 60;
  return `${String(m).padStart(2, "0")}:${String(s).padStart(2, "0")}`;
}
