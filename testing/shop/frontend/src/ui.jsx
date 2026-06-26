import React from 'react'

export function cx(...parts) {
  return parts.filter(Boolean).join(' ')
}

const VARIANTS = {
  primary:
    'bg-neutral-900 text-white hover:bg-neutral-800 focus-visible:outline-neutral-900',
  secondary:
    'bg-white text-neutral-900 ring-1 ring-neutral-950/10 hover:bg-neutral-50 focus-visible:outline-neutral-900',
  ghost:
    'bg-transparent text-neutral-600 hover:bg-neutral-100 hover:text-neutral-900 focus-visible:outline-neutral-900'
}

const SIZES = {
  md: 'px-4 py-2.5 text-sm',
  sm: 'px-3 py-1.5 text-sm'
}

export function Button({ variant = 'primary', size = 'md', className, type = 'button', ...props }) {
  return (
    <button
      type={type}
      className={cx(
        'inline-flex items-center justify-center gap-2 rounded-lg font-medium transition focus-visible:outline-2 focus-visible:outline-offset-2 disabled:pointer-events-none disabled:opacity-50',
        VARIANTS[variant],
        SIZES[size],
        className
      )}
      {...props}
    />
  )
}

export function IconButton({ className, label, type = 'button', children, ...props }) {
  return (
    <button
      type={type}
      aria-label={label}
      className={cx(
        'relative inline-flex size-9 items-center justify-center rounded-lg text-neutral-600 transition hover:bg-neutral-100 hover:text-neutral-900 focus-visible:outline-2 focus-visible:outline-offset-2 focus-visible:outline-neutral-900',
        className
      )}
      {...props}
    >
      {children}
      <span
        className="absolute top-1/2 left-1/2 size-[max(100%,3rem)] -translate-1/2 pointer-fine:hidden"
        aria-hidden="true"
      />
    </button>
  )
}

export function Logo({ className }) {
  return (
    <svg viewBox="0 0 32 32" fill="none" aria-hidden="true" className={className}>
      <rect width="32" height="32" rx="9" className="fill-neutral-900" />
      <path
        d="M11 13h10l-.7 9.3a1.4 1.4 0 0 1-1.4 1.3h-5.8a1.4 1.4 0 0 1-1.4-1.3L11 13Z"
        className="stroke-white"
        strokeWidth="1.5"
        strokeLinejoin="round"
      />
      <path d="M13.4 13v-1.1a2.6 2.6 0 0 1 5.2 0V13" className="stroke-white" strokeWidth="1.5" strokeLinecap="round" />
    </svg>
  )
}

export function IconBag({ className }) {
  return (
    <svg viewBox="0 0 24 24" fill="none" aria-hidden="true" className={className}>
      <path
        d="M6 8h12l-.8 11.2a1.6 1.6 0 0 1-1.6 1.5H8.4a1.6 1.6 0 0 1-1.6-1.5L6 8Z"
        stroke="currentColor"
        strokeWidth="1.6"
        strokeLinejoin="round"
      />
      <path d="M9 8V6.5a3 3 0 0 1 6 0V8" stroke="currentColor" strokeWidth="1.6" strokeLinecap="round" />
    </svg>
  )
}

export function IconStar({ className }) {
  return (
    <svg viewBox="0 0 20 20" fill="currentColor" aria-hidden="true" className={className}>
      <path d="M10 1.8l2.4 4.9 5.4.8-3.9 3.8.9 5.4-4.8-2.5-4.8 2.5.9-5.4L2.5 7.5l5.4-.8L10 1.8Z" />
    </svg>
  )
}

export function IconClose({ className }) {
  return (
    <svg viewBox="0 0 24 24" fill="none" aria-hidden="true" className={className}>
      <path d="M6 6l12 12M18 6 6 18" stroke="currentColor" strokeWidth="1.6" strokeLinecap="round" />
    </svg>
  )
}

export function IconCheck({ className }) {
  return (
    <svg viewBox="0 0 24 24" fill="none" aria-hidden="true" className={className}>
      <path d="M5 12.5 10 17.5 19 7" stroke="currentColor" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round" />
    </svg>
  )
}

export function IconArrowRight({ className }) {
  return (
    <svg viewBox="0 0 24 24" fill="none" aria-hidden="true" className={className}>
      <path d="M5 12h14M13 6l6 6-6 6" stroke="currentColor" strokeWidth="1.6" strokeLinecap="round" strokeLinejoin="round" />
    </svg>
  )
}

export function IconEye({ className }) {
  return (
    <svg viewBox="0 0 24 24" fill="none" aria-hidden="true" className={className}>
      <path
        d="M2.5 12S6 5.5 12 5.5 21.5 12 21.5 12 18 18.5 12 18.5 2.5 12 2.5 12Z"
        stroke="currentColor"
        strokeWidth="1.6"
        strokeLinejoin="round"
      />
      <circle cx="12" cy="12" r="2.8" stroke="currentColor" strokeWidth="1.6" />
    </svg>
  )
}

export function IconLock({ className }) {
  return (
    <svg viewBox="0 0 24 24" fill="none" aria-hidden="true" className={className}>
      <rect x="5" y="10.5" width="14" height="9.5" rx="2" stroke="currentColor" strokeWidth="1.6" />
      <path d="M8 10.5V8a4 4 0 0 1 8 0v2.5" stroke="currentColor" strokeWidth="1.6" strokeLinecap="round" />
    </svg>
  )
}

export function Stars({ count, className }) {
  return (
    <span className={cx('inline-flex items-center gap-1 text-neutral-500', className)}>
      <IconStar className="size-3.5 text-amber-400" />
      <span className="tabular-nums">{count}</span>
    </span>
  )
}
