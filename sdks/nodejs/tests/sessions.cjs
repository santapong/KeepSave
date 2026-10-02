const { test } = require('node:test');
const assert = require('node:assert/strict');
const { KeepSaveClient } = require('../dist/index.js');
const response = (data, status = 200) => ({ status, ok: status < 400, json: async () => data });

test('identity replacement and protected denial clear caches', async () => {
  const original = global.fetch;
  let reads = 0;
  global.fetch = async (url) => {
    if (url.endsWith('/auth/login')) return response({ token: 'new-session' });
    if (url.includes('/secrets?')) return response({ secrets: [{ value: `revision-${++reads}` }] });
    return response(null, 401);
  };
  try {
    const client = new KeepSaveClient({ baseUrl: 'https://synthetic.invalid', token: 'old-session', maxRetries: 0 });
    await client.listSecrets('p', 'alpha'); await client.listSecrets('p', 'alpha'); assert.equal(reads, 1);
    await client.login('synthetic@example.invalid', 'synthetic'); await client.listSecrets('p', 'alpha'); assert.equal(reads, 2);
    await assert.rejects(client.getProject('denied'), (error) => error.code === 401);
    assert.equal(client.token, null); assert.equal(client.cache.store.size, 0);
  } finally { global.fetch = original; }
});

test('human session controls require confirmed revocation and retain distinct API key', async () => {
  const original = global.fetch;
  let unavailable = true;
  global.fetch = async (url, options) => {
    assert.equal(options.headers.Authorization, 'Bearer human-session'); assert.equal(options.headers['X-API-Key'], undefined);
    return url.endsWith('/auth/logout') ? response({ error: 'unavailable' }, unavailable ? 503 : 204) : response({ sessions: [] });
  };
  try {
    const client = new KeepSaveClient({ baseUrl: 'https://synthetic.invalid', token: 'human-session', apiKey: 'api-key', maxRetries: 0 });
    await client.listSessions(); client.cache.set('fixture', ['synthetic']);
    await assert.rejects(client.logout()); assert.equal(client.token, 'human-session'); assert.deepEqual(client.cache.get('fixture'), ['synthetic']);
    unavailable = false; await client.logout(); assert.equal(client.token, null); assert.equal(client.apiKey, 'api-key'); assert.equal(client.cache.get('fixture'), undefined);
  } finally { global.fetch = original; }
});


test('an earlier account response cannot repopulate a newer account cache', async () => {
  const original = global.fetch; let finishOld, reads = 0;
  global.fetch = async () => {
    reads++;
    if (reads === 1) return new Promise((resolve) => { finishOld = resolve; });
    return response({ secrets: [{ value: 'new-account' }] });
  };
  try {
    const client = new KeepSaveClient({ baseUrl: 'https://synthetic.invalid', token: 'old-session', maxRetries: 0 });
    const old = client.listSecrets('p', 'alpha'); client.setToken('new-session');
    await client.listSecrets('p', 'alpha'); finishOld(response({ secrets: [{ value: 'old-account' }] })); await old;
    const current = await client.listSecrets('p', 'alpha'); assert.equal(current[0].value, 'new-account'); assert.equal(reads, 2);
  } finally { global.fetch = original; }
});
