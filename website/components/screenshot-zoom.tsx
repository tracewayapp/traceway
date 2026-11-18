"use client";

import Image from "next/image";
import { useState } from "react";
import * as Dialog from "@radix-ui/react-dialog";
import { ChevronLeft, ChevronRight, Expand, X } from "lucide-react";
import styles from "./screenshot-zoom.module.css";

type Screenshot = {
  src: string;
  width: number;
  height: number;
  alt: string;
  label: string;
  title: string;
  description: string;
};

export function ScreenshotZoom({ images, index = 0, eager = false }: {
  images: readonly Screenshot[];
  index?: number;
  eager?: boolean;
}) {
  const [open, setOpen] = useState(false);
  const [selected, setSelected] = useState(index);
  const preview = images[index];
  const image = images[selected];
  const step = (delta: number) => setSelected(current => (current + delta + images.length) % images.length);
  const controlClass = "inline-flex h-11 w-11 shrink-0 cursor-pointer items-center justify-center rounded-full bg-white/10 text-white transition-colors hover:bg-white/20 focus-visible:outline-2 focus-visible:outline-offset-2 focus-visible:outline-white";

  return (
    <Dialog.Root open={open} onOpenChange={setOpen}>
      <Dialog.Trigger asChild>
        <button
          type="button"
          className="group relative block w-full cursor-zoom-in text-left focus-visible:outline-2 focus-visible:outline-offset-[-4px] focus-visible:outline-a2"
          onClick={() => setSelected(index)}
          aria-label={`Enlarge: ${preview.label}`}
        >
          <Image
            src={preview.src}
            alt={preview.alt}
            width={preview.width}
            height={preview.height}
            sizes="(max-width: 1280px) 100vw, 1240px"
            loading={eager ? "eager" : "lazy"}
            className="block h-auto w-full"
          />
          <span className="absolute right-3 bottom-3 flex items-center gap-2 rounded-md border border-white/15 bg-[#05070c] px-3 py-2 text-xs text-white/80 group-hover:text-white">
            <Expand className="h-3.5 w-3.5" aria-hidden="true" />
            Full size
          </span>
        </button>
      </Dialog.Trigger>
      <Dialog.Portal>
        <Dialog.Overlay className={styles.overlay} onClick={event => {
          if (event.target === event.currentTarget) setOpen(false);
        }}>
          <Dialog.Content className={styles.panel} onKeyDown={event => {
            if (event.key === "ArrowRight" || event.key === "ArrowLeft") {
              event.preventDefault();
              step(event.key === "ArrowRight" ? 1 : -1);
            }
          }}>
            <div className="mb-3 flex items-center justify-between text-sm text-white/70">
              <span aria-live="polite">{selected + 1} / {images.length}</span>
              <Dialog.Close className={controlClass} aria-label="Close image viewer">
                <X className="h-5 w-5" aria-hidden="true" />
              </Dialog.Close>
            </div>
            <div className="overflow-hidden rounded-xl border border-white/15 bg-[#0a0d14]">
              <Image
                key={image.src}
                src={image.src}
                alt={image.alt}
                width={image.width}
                height={image.height}
                sizes="100vw"
                loading="eager"
                className="max-h-[60dvh] w-full object-contain"
              />
            </div>
            <div className="mt-4 flex flex-col items-start justify-between gap-4 sm:flex-row sm:gap-6">
              <div className="min-w-0" aria-live="polite">
                <Dialog.Title className="text-xl font-semibold text-white">{image.title}</Dialog.Title>
                <Dialog.Description className="mt-2 max-w-3xl text-[15px] leading-relaxed text-white/70">{image.description}</Dialog.Description>
              </div>
              {images.length > 1 && <div className="flex shrink-0 gap-2">
                <button type="button" className={controlClass} onClick={() => step(-1)} aria-label="Previous screenshot">
                  <ChevronLeft className="h-5 w-5" aria-hidden="true" />
                </button>
                <button type="button" className={controlClass} onClick={() => step(1)} aria-label="Next screenshot">
                  <ChevronRight className="h-5 w-5" aria-hidden="true" />
                </button>
              </div>}
            </div>
          </Dialog.Content>
        </Dialog.Overlay>
      </Dialog.Portal>
    </Dialog.Root>
  );
}
