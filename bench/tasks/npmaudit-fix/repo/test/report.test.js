'use strict';

const test = require('node:test');
const assert = require('node:assert');
const fs = require('node:fs');
const os = require('node:os');
const path = require('node:path');
const http = require('node:http');

const { parseArgs } = require('../src/cli');
const { fetchFeed, fetchText } = require('../src/feed');
const { summarize, requester, writeReport } = require('../src/report');
const { main } = require('../src/index');

const items = [
  { warehouse: 'north', sku: 'A1', units: 4, updatedAt: 1000 },
  { warehouse: 'north', sku: 'A1', units: 6, updatedAt: 4000 },
  { warehouse: 'south', sku: 'B2', units: 3, updatedAt: 2000 },
  { warehouse: 'north', sku: 'C3', units: 1, updatedAt: 3000 },
];

function serve(handler) {
  return new Promise((resolve) => {
    const server = http.createServer(handler);
    server.listen(0, '127.0.0.1', () => {
      resolve({ server, base: `http://127.0.0.1:${server.address().port}` });
    });
  });
}

test('parseArgs reads aliases and defaults', () => {
  assert.deepStrictEqual(parseArgs(['-f', 'http://x/feed', '-v']), {
    feed: 'http://x/feed',
    out: 'reports',
    maxAge: 86400000,
    verbose: true,
  });
  assert.throws(() => parseArgs([]), /--feed is required/);
});

test('summarize groups by warehouse', () => {
  assert.deepStrictEqual(summarize(items, 7200000 + 2000), [
    { warehouse: 'north', skus: 2, units: 11, staleFor: '2h' },
    { warehouse: 'south', skus: 1, units: 3, staleFor: '2h' },
  ]);
});

test('requester reads the user cookie', () => {
  assert.strictEqual(requester('theme=dark; user=ana'), 'ana');
  assert.strictEqual(requester(undefined), 'anonymous');
});

test('writeReport creates nested directories', () => {
  const dir = path.join(fs.mkdtempSync(path.join(os.tmpdir(), 'inv-')), 'a', 'b');
  const { id, file } = writeReport(dir, [], 'ana');
  assert.match(id, /^[A-Za-z0-9_-]{10}$/);
  assert.deepStrictEqual(JSON.parse(fs.readFileSync(file, 'utf8')), { id, user: 'ana', summary: [] });
});

test('fetchFeed and fetchText talk to a local server', async () => {
  const { server, base } = await serve((req, res) => {
    if (req.url === '/old') {
      res.writeHead(302, { location: '/feed' });
      res.end();
      return;
    }
    res.writeHead(req.url === '/feed' ? 200 : 404, { 'content-type': 'application/json' });
    res.end(JSON.stringify(items));
  });
  try {
    assert.deepStrictEqual(await fetchFeed(`${base}/feed`), items);
    await assert.rejects(fetchFeed(`${base}/missing`), /feed request failed: 404/);
    const page = await fetchText(`${base}/old`);
    assert.strictEqual(page.status, 200);
    assert.strictEqual(page.url, `${base}/feed`);
  } finally {
    server.close();
  }
});

test('main writes a report from the feed', async () => {
  const { server, base } = await serve((req, res) => {
    res.writeHead(200, { 'content-type': 'application/json' });
    res.end(JSON.stringify(items));
  });
  try {
    const out = fs.mkdtempSync(path.join(os.tmpdir(), 'inv-'));
    const file = await main(['--feed', `${base}/feed`, '--out', out], { HTTP_COOKIE: 'user=bo' });
    const report = JSON.parse(fs.readFileSync(file, 'utf8'));
    assert.strictEqual(report.user, 'bo');
    assert.strictEqual(report.summary.length, 2);
  } finally {
    server.close();
  }
});
