// Read-only Core capture. Never read pair credentials, argv, terminal text or env.
const fs = require('node:fs');
const path = require('node:path');
const os = require('node:os');
const cp = require('node:child_process');
const readline = require('node:readline');
const crypto = require('node:crypto');
const home = os.homedir();
const hc = process.env.PAIRFOB_ACCEPT_HC || path.join(home, '.local/bin/hc');
const binary = path.join(home, '.local/bin/pairfob');
const node = process.env.PAIRFOB_ACCEPT_NODE || os.hostname();
const child = cp.spawn(hc, ['connect'], { stdio: ['pipe', 'pipe', 'pipe'] });
const pending = new Map();
let next = 0;
const timer = setTimeout(() => fail(new Error('Core capture exceeded 30s')), 30_000);
function fail(error) {
  clearTimeout(timer);
  for (const { reject } of pending.values()) reject(error);
  pending.clear();
  child.kill();
  console.error(`${node}: ${error.message}`);
  process.exitCode = 1;
}
child.on('error', fail);
child.stderr.resume(); // Core stderr can contain unrelated user data; do not persist it.
child.on('exit', () => {
  if (pending.size) fail(new Error('hc connect ended before all reads completed'));
});
readline.createInterface({ input: child.stdout }).on('line', line => {
  let reply;
  try { reply = JSON.parse(line); } catch { return fail(new Error('Invalid Core JSON')); }
  const waiter = pending.get(reply.id);
  if (!waiter) return;
  pending.delete(reply.id);
  if (reply.error) waiter.reject(new Error(`${waiter.method}: ${reply.error.kind || reply.error.code || 'RPC error'}`));
  else waiter.resolve(reply.result);
});
function call(method, params) {
  if (!['hello', 'terminal.list', 'doc.get'].includes(method)) throw new Error('Read-only capture refused method');
  return new Promise((resolve, reject) => {
    const id = ++next;
    pending.set(id, { resolve, reject, method });
    child.stdin.write(JSON.stringify({ jsonrpc: '2.0', id, method, params }) + '\n');
  });
}
function tree(n) {
  if (!n) return undefined;
  return { id: n.id, type: n.type, hidden: n.hidden, active: n.active, size: n.size,
    content: { terminal: n.content?.terminal }, children: n.children?.map(tree) };
}
(async () => {
  const hello = await call('hello', { framed: false });
  const list = await call('terminal.list', { all: false });
  if (!Array.isArray(list.terminals)) throw new Error('Missing terminal.list.terminals');
  const terminals = list.terminals.map(t => ({
    address: t.address, state: t.state, cwd: t.cwd, cols: t.cols, rows: t.rows,
    name: t.name, title: t.title, status: t.status,
    labels: Object.fromEntries(['herdr.space', 'herdr.tab', 'where', 'pairfob.label']
      .filter(k => t.labels?.[k] !== undefined).map(k => [k, t.labels[k]])),
    foreground: t.foreground ? { name: t.foreground.name, cwd: t.foreground.cwd } : undefined,
  }));
  const docs = {};
  const machines = [...new Set(['', ...terminals.map(t => t.address.includes('/') ? t.address.split('/')[0] : '')])].sort();
  for (const machine of machines) {
    const doc = await call('doc.get', { name: 'herdr-app', ...(machine ? { on: machine } : {}) });
    const value = typeof doc.value === 'string' ? JSON.parse(doc.value) : doc.value;
    docs[machine] = { version: doc.version, value: { spaces: (value?.spaces || []).map(sp => ({
      id: sp.id, name: sp.name, folder: sp.folder,
      tabs: (sp.tabs || []).map(tab => ({ id: tab.id, name: tab.name, root: tree(tab.root) })),
    })) } };
  }
  const build = cp.execFileSync(binary, ['--version'], { encoding: 'utf8', timeout: 10_000 }).trim();
  const result = { schema: 1, node, host: os.hostname(), capturedAt: new Date().toISOString(),
    binary: { build, sha256: crypto.createHash('sha256').update(fs.readFileSync(binary)).digest('hex') },
    hello: { version: hello.version, generation: hello.generation }, terminals, docs };
  process.stdout.write(JSON.stringify(result, null, 2) + '\n');
  clearTimeout(timer);
  child.stdin.end();
  child.kill();
})().catch(fail);
