'use strict';

async function request(base, method, path, body) {
  const res = await fetch(base + path, {
    method,
    headers: { accept: 'application/json', ...(body ? { 'content-type': 'application/json' } : {}) },
    body: body ? JSON.stringify(body) : undefined,
  });
  if (!res.ok) {
    throw new Error(`${method} ${base}${path} answered ${res.status}`);
  }
  return res.json();
}

module.exports = { request };
