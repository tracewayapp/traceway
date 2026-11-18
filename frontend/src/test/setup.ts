/// <reference types="vitest/jsdom" />
import { vi } from 'vitest';

// Node's native storage globals prevent Vitest from installing jsdom's browser storage.
vi.stubGlobal('localStorage', jsdom.window.localStorage);
vi.stubGlobal('sessionStorage', jsdom.window.sessionStorage);
