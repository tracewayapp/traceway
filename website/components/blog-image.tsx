"use client";

import * as Dialog from "@radix-ui/react-dialog";
import { X } from "lucide-react";

/**
 * Markdown images in a post. The reading column is narrow, so dashboard
 * screenshots open at full size on click.
 */
export function BlogImage({ src, alt }: { src?: string; alt?: string }) {
  if (!src) return null;
  return (
    <Dialog.Root>
      <Dialog.Trigger asChild>
        <button
          type="button"
          className="blog-image"
          aria-label={alt ? `Enlarge image: ${alt}` : "Enlarge image"}
        >
          {/* eslint-disable-next-line @next/next/no-img-element */}
          <img src={src} alt={alt ?? ""} loading="lazy" />
        </button>
      </Dialog.Trigger>
      <Dialog.Portal>
        <Dialog.Overlay className="blog-lightbox-overlay" />
        <Dialog.Content className="blog-lightbox" aria-describedby={undefined}>
          <Dialog.Title className="sr-only">{alt || "Image"}</Dialog.Title>
          <Dialog.Close asChild>
            {/* eslint-disable-next-line @next/next/no-img-element */}
            <img src={src} alt={alt ?? ""} />
          </Dialog.Close>
          <Dialog.Close className="blog-lightbox-close" aria-label="Close">
            <X className="h-4 w-4" aria-hidden="true" />
          </Dialog.Close>
        </Dialog.Content>
      </Dialog.Portal>
    </Dialog.Root>
  );
}
