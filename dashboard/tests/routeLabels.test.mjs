import assert from 'node:assert/strict';
import { readFileSync } from 'node:fs';
import { createRequire } from 'node:module';
import test from 'node:test';
import { transformSync } from 'esbuild';

const nodeRequire = createRequire(import.meta.url);

function load(path, stubs = {}, define = {}) {
  const source = readFileSync(new URL(path, import.meta.url), 'utf8');
  const code = transformSync(source, { loader: path.endsWith('.tsx') ? 'tsx' : 'ts', format: 'cjs', jsx: 'automatic', define }).code;
  const mod = { exports: {} };
  new Function('require', 'module', 'exports', code)((name) => stubs[name] ?? requireModule(name), mod, mod.exports);
  return mod.exports;
}

function requireModule(name) {
  return nodeRequire(name);
}

const { routeLabelDraft } = load('../src/lib/routeLabels.ts');

test('route draft preserves other policy fields and source object', () => {
  const original = { mode: 'shadow', rules: [{ id: 'keep' }], allowlist: ['192.0.2.0/24'], routeClasses: { '/old': 'login' } };
  const next = routeLabelDraft(original, '/account/signin', 'login');
  assert.deepEqual(next.routeClasses, { '/old': 'login', '/account/signin': 'login' });
  assert.deepEqual(next.rules, original.rules);
  assert.deepEqual(next.allowlist, original.allowlist);
  assert.deepEqual(original.routeClasses, { '/old': 'login' });
  assert.deepEqual(routeLabelDraft(next, '/old', null).routeClasses, { '/account/signin': 'login' });
});

test('route draft refuses active policy and ambiguous paths', () => {
  assert.throws(() => routeLabelDraft({ mode: 'enforce', rules: [] }, '/login', 'login'), /operator review/);
  for (const path of ['/a/../login', '/a%2flogin', '/login?next=x', 'relative']) {
    assert.throws(() => routeLabelDraft(null, path, 'login'), /exact path/, path);
  }
  assert.throws(() => routeLabelDraft({ mode: 'shadow', rules: [], routeClasses: { '/login': 'login' } }, '/login', 'login'), /already tagged/);
});

function apiWith(fetcher) {
  globalThis.fetch = fetcher;
  return load('../src/lib/api.ts', { './supabaseClient': { supabase: { auth: { getSession: async () => ({ data: { session: { access_token: 'owned-token' } } }) } } } }, {
    'import.meta.env.VITE_API_BASE_URL': '"https://api.example/api/v1"',
    'import.meta.env.DEV': 'false',
  });
}

test('policy API scopes path to selected tenant, sends token and expected version', async () => {
  let called = 0;
  const api = apiWith(async (url, options) => {
    called++;
    assert.equal(url, 'https://api.example/api/v1/domains/tenant-a/policy');
    assert.equal(options.headers.Authorization, 'Bearer owned-token');
    if (options.method === 'PUT') {
      assert.deepEqual(JSON.parse(options.body), { expectedVersion: 3, document: { mode: 'shadow', rules: [], routeClasses: { '/signin': 'login' } } });
    }
    return { ok: true, status: 200, json: async () => ({ success: true, data: { version: 4, document: { mode: 'shadow', rules: [] } } }) };
  });
  assert.equal((await api.getTenantPolicy('tenant-a')).version, 4);
  assert.equal((await api.saveTenantPolicy('tenant-a', 3, { mode: 'shadow', rules: [], routeClasses: { '/signin': 'login' } })).version, 4);
  assert.equal(called, 2);
});

test('missing policy is a new draft; network and conflict errors are not hidden', async () => {
  const api = apiWith(async () => ({ ok: false, status: 404, json: async () => ({ success: false, error: 'not found' }) }));
  assert.equal(await api.getTenantPolicy('new'), null);
  const conflict = apiWith(async () => ({ ok: false, status: 409, json: async () => ({ success: false, error: 'version conflict' }) }));
  await assert.rejects(conflict.saveTenantPolicy('t', 1, { mode: 'shadow', rules: [] }), (error) => error.status === 409);
});

function renderSettings(revision, loadedTenantId = 'tenant-a') {
  const values = [revision, false, loadedTenantId, false, null, null, '', 'login'];
  const fakeReact = {
    useState: (initial) => { const next = values.shift(); return [next === undefined ? initial : next, () => {}]; },
    useRef: (current) => ({ current }),
    useEffect: () => {},
  };
  const { default: Settings } = load('../src/pages/ProtectionSettings.tsx', {
    react: fakeReact,
    'react-router-dom': { useOutletContext: () => ({ selectedDomain: { id: 'tenant-a', domain: 'client.example' }, domainsLoading: false }) },
    'lucide-react': { AlertCircle: () => null, Shield: () => null },
    '../lib/api': { ApiError: class ApiError extends Error {}, getTenantPolicy: async () => null, saveTenantPolicy: async () => null },
    '../lib/routeLabels': { routeLabelDraft },
  });
  const React = nodeRequire('react');
  return nodeRequire('react-dom/server').renderToStaticMarkup(React.createElement(Settings));
}

test('settings shows draft editor only for shadow policy', () => {
  const shadow = renderSettings({ version: 1, document: { mode: 'shadow', rules: [], routeClasses: { '/account/signin': 'login' } } });
  assert.match(shadow, /\/account\/signin/);
  assert.match(shadow, /Save shadow draft/);
  const active = renderSettings({ version: 2, document: { mode: 'enforce', rules: [], routeClasses: { '/account/signin': 'login' } } });
  assert.match(active, /This policy is active/);
  assert.doesNotMatch(active, /Save shadow draft/);
});

test('domain switch never renders the previous tenant route draft', () => {
  const html = renderSettings({ version: 1, document: { mode: 'shadow', rules: [], routeClasses: { '/private-login': 'login' } } }, 'tenant-b');
  assert.doesNotMatch(html, /private-login/);
  assert.doesNotMatch(html, /Save shadow draft/);
});
