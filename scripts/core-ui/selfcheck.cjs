// Negative controls: prove the acceptance oracle rejects actual regressions.
// Mutate only in-memory copies of the private captures, never live Core state.
const fs = require('node:fs');
const path = require('node:path');
const assert = require('node:assert/strict');
const { expectations } = require('./oracle.cjs');
const root = process.argv[2];
const caught = new Set();
for (const file of fs.readdirSync(path.join(root,'captures')).filter(f => f.endsWith('.json'))) {
  const capture = JSON.parse(fs.readFileSync(path.join(root,'captures',file)));
  const snapshot = JSON.parse(fs.readFileSync(path.join(root,'projected',file)));
  const expected = expectations(capture,snapshot);
  function rejects(name, mutate, pattern) {
    const bad = structuredClone(snapshot);
    mutate(bad);
    assert.throws(() => expectations(capture,bad),pattern,`${name}: oracle falsely passed`);
    caught.add(name);
  }
  rejects('missing-pane',s => s.panes.pop(),/missing\/extra pane/);
  rejects('wrong-project-name',s => s.workspaces[0].label = 'wrong',/project name/);
  const custom = expected.find(e => e.title && e.workspaceId.includes(':space:'));
  if (custom) rejects('stale-App-title',s => {
    const p = s.panes.find(p => p.pane_id === custom.paneId);
    s.tabs.find(t => t.tab_id === p.tab_id).label = 'stale title';
  },/lost custom App title/);
  const remote = expected.find(e => e.machine);
  if (remote) rejects('remote-local-root',s => s.panes.find(p => p.pane_id === remote.paneId).cwd='/home/wrong',/remote path became/);
}
assert.equal(caught.size,4,'negative controls lack named or remote samples');
// Same basename in different real-world checkout shapes must not merge, even
// if a buggy renderer reports a perfectly plausible project name for both.
const terminals = ['/home/u/checkout-a/repo','/home/u/checkout-b/repo'].map((cwd,i) => ({address:`t${i}`,state:'live',cwd,labels:{}}));
const broken = {workspaces:[{workspace_id:'wrong-shared',label:'repo'}],
  tabs:terminals.map(t=>({tab_id:t.address,workspace_id:'wrong-shared',label:''})),
  panes:terminals.map(t=>({pane_id:t.address,terminal_id:t.address,tab_id:t.address,workspace_id:'wrong-shared',cwd:t.cwd}))};
assert.throws(()=>expectations({schema:1,terminals,docs:{}},broken),/fallback grouping identity/);
console.log(`Negative controls caught: ${[...caught].join(', ')}, merged-checkouts`);
