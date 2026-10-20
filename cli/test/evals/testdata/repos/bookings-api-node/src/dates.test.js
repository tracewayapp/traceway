import { test } from 'node:test';
import assert from 'node:assert/strict';
import { nightsBetween } from './dates.js';

test('nights between two dates', () => {
  assert.equal(nightsBetween('2026-11-02', '2026-11-05'), 3);
  assert.equal(nightsBetween('2026-11-05', '2026-11-02'), -3);
  assert.equal(nightsBetween('soon', '2026-11-02'), 0);
});
