"use client";

import { useEffect, useRef, useState } from "react";
import { Play, Pause, Volume2, VolumeX, Maximize, Minimize } from "lucide-react";

import { cn } from "@/lib/utils";

type Props = {
  src: string;
  poster?: string;
  className?: string;
};

const STYLES = `
.vp {
  position: relative;
  aspect-ratio: 16 / 9;
  width: 100%;
  border-radius: 12px;
  overflow: hidden;
  background: #05070c;
  border: 1px solid rgba(255, 255, 255, 0.08);
  box-shadow: 0 36px 72px -36px rgba(10, 14, 24, 0.6);
  isolation: isolate;
}
.vp-video {
  position: absolute;
  inset: 0;
  width: 100%;
  height: 100%;
  object-fit: contain;
  display: block;
  background: #05070c;
  cursor: pointer;
}
.vp-bigplay {
  position: absolute;
  top: 50%;
  left: 50%;
  transform: translate(-50%, -50%);
  z-index: 2;
  display: grid;
  place-items: center;
  width: 80px;
  height: 80px;
  border: 0;
  padding: 0;
  border-radius: 9999px;
  cursor: pointer;
  background: linear-gradient(135deg, #7c5cff, #00d4ff);
  box-shadow:
    0 12px 34px -8px rgba(124, 92, 255, 0.65),
    0 0 0 10px rgba(255, 255, 255, 0.06);
  opacity: 0;
  pointer-events: none;
  transition: opacity 0.25s ease, transform 0.25s cubic-bezier(0.16, 1, 0.3, 1);
}
.vp[data-paused="true"] .vp-bigplay {
  opacity: 1;
  pointer-events: auto;
}
.vp-bigplay:hover {
  transform: translate(-50%, -50%) scale(1.07);
}
.vp-bigplay svg {
  width: 32px;
  height: 32px;
  margin-left: 4px;
  color: #fff;
  fill: #fff;
}
.vp-controls {
  position: absolute;
  left: 0;
  right: 0;
  bottom: 0;
  z-index: 3;
  display: flex;
  align-items: center;
  gap: 12px;
  padding: 28px 14px 12px;
  background: linear-gradient(to top, rgba(5, 7, 12, 0.85), transparent);
  opacity: 0;
  transform: translateY(6px);
  transition: opacity 0.2s ease, transform 0.2s ease;
}
.vp:hover .vp-controls,
.vp:focus-within .vp-controls,
.vp[data-paused="true"] .vp-controls {
  opacity: 1;
  transform: translateY(0);
}
.vp-btn {
  display: grid;
  place-items: center;
  width: 32px;
  height: 32px;
  flex: none;
  border: 0;
  padding: 0;
  border-radius: 7px;
  cursor: pointer;
  background: transparent;
  color: #f4f6fb;
  transition: background 0.15s ease, color 0.15s ease;
}
.vp-btn:hover {
  background: rgba(255, 255, 255, 0.12);
}
.vp-btn svg {
  width: 18px;
  height: 18px;
}
.vp-progress {
  position: relative;
  flex: 1 1 auto;
  height: 16px;
  display: flex;
  align-items: center;
  cursor: pointer;
  touch-action: none;
}
.vp-progress-track {
  position: relative;
  width: 100%;
  height: 4px;
  border-radius: 9999px;
  background: rgba(255, 255, 255, 0.22);
  overflow: hidden;
  transition: height 0.15s ease;
}
.vp-progress:hover .vp-progress-track {
  height: 6px;
}
.vp-progress-fill {
  position: absolute;
  inset: 0 auto 0 0;
  height: 100%;
  border-radius: 9999px;
  background: linear-gradient(90deg, #7c5cff, #00d4ff);
}
.vp-progress-thumb {
  position: absolute;
  top: 50%;
  width: 12px;
  height: 12px;
  border-radius: 9999px;
  background: #fff;
  box-shadow: 0 1px 4px rgba(0, 0, 0, 0.45);
  transform: translate(-50%, -50%) scale(0);
  transition: transform 0.15s ease;
  pointer-events: none;
}
.vp-progress:hover .vp-progress-thumb {
  transform: translate(-50%, -50%) scale(1);
}
.vp-time {
  flex: none;
  font-family: var(--font-mono, ui-monospace, monospace);
  font-size: 12px;
  font-variant-numeric: tabular-nums;
  color: #c9d0dd;
  letter-spacing: 0.02em;
}
`;

function fmt(seconds: number) {
  if (!Number.isFinite(seconds) || seconds < 0) seconds = 0;
  const m = Math.floor(seconds / 60);
  const s = Math.floor(seconds % 60);
  return `${m}:${s.toString().padStart(2, "0")}`;
}

export function VideoPlayer({ src, poster, className }: Props) {
  const videoRef = useRef<HTMLVideoElement>(null);
  const containerRef = useRef<HTMLDivElement>(null);
  const userPausedRef = useRef(false);

  const [paused, setPaused] = useState(true);
  const [muted, setMuted] = useState(true);
  const [current, setCurrent] = useState(0);
  const [duration, setDuration] = useState(0);
  const [fullscreen, setFullscreen] = useState(false);

  useEffect(() => {
    const video = videoRef.current;
    if (!video) return;
    video.muted = true;

    const onTime = () => setCurrent(video.currentTime);
    const onMeta = () => setDuration(video.duration);
    const onPlay = () => setPaused(false);
    const onPause = () => setPaused(true);
    const onVolume = () => setMuted(video.muted);
    if (video.readyState >= 1 && Number.isFinite(video.duration)) setDuration(video.duration);
    video.addEventListener("timeupdate", onTime);
    video.addEventListener("loadedmetadata", onMeta);
    video.addEventListener("durationchange", onMeta);
    video.addEventListener("play", onPlay);
    video.addEventListener("pause", onPause);
    video.addEventListener("volumechange", onVolume);

    const io = new IntersectionObserver(
      ([entry]) => {
        if (entry.isIntersecting && entry.intersectionRatio >= 0.4) {
          if (!userPausedRef.current) video.play().catch(() => {});
        } else if (!video.paused) {
          video.pause();
        }
      },
      { threshold: [0, 0.4, 1] }
    );
    if (containerRef.current) io.observe(containerRef.current);

    const onFs = () => setFullscreen(document.fullscreenElement === containerRef.current);
    document.addEventListener("fullscreenchange", onFs);

    return () => {
      video.removeEventListener("timeupdate", onTime);
      video.removeEventListener("loadedmetadata", onMeta);
      video.removeEventListener("durationchange", onMeta);
      video.removeEventListener("play", onPlay);
      video.removeEventListener("pause", onPause);
      video.removeEventListener("volumechange", onVolume);
      document.removeEventListener("fullscreenchange", onFs);
      io.disconnect();
    };
  }, []);

  function togglePlay() {
    const video = videoRef.current;
    if (!video) return;
    if (video.paused) {
      userPausedRef.current = false;
      video.play().catch(() => {});
    } else {
      userPausedRef.current = true;
      video.pause();
    }
  }

  function toggleMute() {
    const video = videoRef.current;
    if (!video) return;
    video.muted = !video.muted;
  }

  function seekFromPointer(e: React.PointerEvent<HTMLDivElement>) {
    const video = videoRef.current;
    if (!video || !duration) return;
    const rect = e.currentTarget.getBoundingClientRect();
    const ratio = Math.min(1, Math.max(0, (e.clientX - rect.left) / rect.width));
    video.currentTime = ratio * duration;
    setCurrent(video.currentTime);
  }

  function onProgressDown(e: React.PointerEvent<HTMLDivElement>) {
    e.currentTarget.setPointerCapture(e.pointerId);
    seekFromPointer(e);
  }

  function onProgressMove(e: React.PointerEvent<HTMLDivElement>) {
    if (e.currentTarget.hasPointerCapture(e.pointerId)) seekFromPointer(e);
  }

  function toggleFullscreen() {
    const el = containerRef.current;
    if (!el) return;
    if (document.fullscreenElement === el) {
      document.exitFullscreen().catch(() => {});
    } else {
      el.requestFullscreen().catch(() => {});
    }
  }

  const pct = duration ? (current / duration) * 100 : 0;

  return (
    <div ref={containerRef} className={cn("vp", className)} data-paused={paused}>
      <style>{STYLES}</style>
      <video
        ref={videoRef}
        className="vp-video"
        src={src}
        poster={poster}
        muted
        loop
        playsInline
        preload="metadata"
        onClick={togglePlay}
      />

      <button
        type="button"
        className="vp-bigplay"
        onClick={togglePlay}
        aria-label="Play"
      >
        <Play />
      </button>

      <div className="vp-controls">
        <button
          type="button"
          className="vp-btn"
          onClick={togglePlay}
          aria-label={paused ? "Play" : "Pause"}
        >
          {paused ? <Play /> : <Pause />}
        </button>

        <div
          className="vp-progress"
          onPointerDown={onProgressDown}
          onPointerMove={onProgressMove}
          role="slider"
          aria-label="Seek"
          aria-valuemin={0}
          aria-valuemax={Math.round(duration)}
          aria-valuenow={Math.round(current)}
          tabIndex={0}
        >
          <div className="vp-progress-track">
            <div className="vp-progress-fill" style={{ width: `${pct}%` }} />
          </div>
          <div className="vp-progress-thumb" style={{ left: `${pct}%` }} />
        </div>

        <span className="vp-time">
          {fmt(current)} / {fmt(duration)}
        </span>

        <button
          type="button"
          className="vp-btn"
          onClick={toggleMute}
          aria-label={muted ? "Unmute" : "Mute"}
        >
          {muted ? <VolumeX /> : <Volume2 />}
        </button>

        <button
          type="button"
          className="vp-btn"
          onClick={toggleFullscreen}
          aria-label={fullscreen ? "Exit fullscreen" : "Fullscreen"}
        >
          {fullscreen ? <Minimize /> : <Maximize />}
        </button>
      </div>
    </div>
  );
}
