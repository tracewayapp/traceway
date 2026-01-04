# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Project Overview

This is the frontend for Traceway, an error tracking and monitoring application. Built with SvelteKit 2 using Svelte 5's new runes API, styled with Tailwind CSS v4, and uses shadcn-svelte for UI components.

## Build & Development Commands

```bash
# Development server
npm run dev

# Production build (outputs to /build with static adapter)
npm run build

# Preview production build locally
npm run preview

# Type checking
npm run check
npm run check:watch

# Linting & formatting
npm run lint          # Check formatting and run ESLint
npm run format        # Format all files with Prettier
```

## Architecture

### SvelteKit Configuration
- **SSR disabled**: `ssr = false` in routes/+layout.ts
- **Prerendering enabled**: `prerender = true` for static site generation
- **Adapter**: Uses `@sveltejs/adapter-static` with `fallback: 'index.html'` for SPA behavior
- **API Proxy**: Vite dev server proxies `/api` requests to `http://localhost:8082`

### State Management (Svelte 5 Runes)
The app uses Svelte 5's new runes-based reactivity system instead of stores:

- **`src/lib/state/auth.svelte.ts`**: Authentication state using `$state` rune
  - Token stored in localStorage as `APP_TOKEN`
  - `$effect` for localStorage synchronization
  - `isAuthenticated` derived state using `$derived`

- **`src/lib/state/theme.svelte.ts`**: Theme state management
  - Tracks dark/light mode with DOM class observation
  - Persists to localStorage
  - Functions: `initTheme()`, `toggleTheme()`

### API Client (`src/lib/api.ts`)
Centralized API wrapper that:
- Prefixes all requests with `/api`
- Auto-includes Authorization header from authState
- Handles 401s by logging out and redirecting to `/login`
- Exports methods: `api.get()`, `api.post()`, `api.put()`, `api.delete()`

### Layout & Navigation
- **Root layout** (`src/routes/+layout.svelte`): Conditionally renders sidebar for authenticated users
  - Unauthenticated users see full-screen content (login page)
  - Authenticated users get sidebar + header with theme toggle and logout
- **Sidebar** (`src/lib/components/app-sidebar.svelte`): Navigation with theme-aware logo switching

### UI Components
Uses shadcn-svelte with configuration in `components.json`:
- Component path: `src/lib/components/ui/*`
- Registry: https://shadcn-svelte.com/registry
- Aliases configured: `$lib/components`, `$lib/utils`, `$lib/components/ui`, `$lib/hooks`

## Key Patterns

### Svelte 5 Runes
This codebase uses Svelte 5's modern reactivity:
- Use `$state()` for reactive state (not `writable()` stores)
- Use `$derived()` for computed values (not `derived()` stores)
- Use `$effect()` for side effects (not `$:` reactive statements)
- Use `{@render children()}` syntax in layouts

### Component Imports
```typescript
// UI components use namespace imports
import * as Sidebar from "$lib/components/ui/sidebar";

// Icons from lucide-svelte
import { Sun, Moon, LogOut } from "@lucide/svelte";
```

### Authentication Flow
1. Token stored in `authState.token`
2. API client auto-includes token in Authorization header
3. 401 responses trigger automatic logout + redirect to `/login`
4. Layout conditionally shows sidebar based on `authState.isAuthenticated`

## Routes
- `/` - Root page (redirects or shows dashboard based on auth)
- `/login` - Login page (public)
- `/issues` - Issues dashboard (protected)

## Backend Integration
Backend runs on `localhost:8082` during development. All API requests go through `/api` prefix which Vite proxies in dev mode.
