// Expectations come from captured Core documents, independently of Go/PWA
// projection. Check all identities/groups and every explicit user name; generic
// OSC title cleanup remains the PWA's responsibility.
const assert = require('node:assert/strict');
const path = require('node:path').posix;
const { createHash } = require('node:crypto');
function leaves(root, depth = 0) {
  assert(depth <= 32, 'App document exceeds depth limit');
  return [...(root?.type === 'block' && root.content?.terminal ? [root.content.terminal] : []),
    ...(root?.children || []).flatMap(child => leaves(child, depth + 1))];
}
const machineOf = address => address.includes('/') ? address.split('/')[0] : '';
const whereName = terminal => {
  const parts = (terminal.labels?.where || '').trim().split(' / ');
  return parts.length <= 2 ? parts[0].trim() : '';
};
const customTab = name => {
  const s = (name || '').trim();
  return s && !/^\d+$/.test(s) && !['main', 'default', 'tab', 'new tab', 'terminal', 'shell'].includes(s.toLowerCase()) ? s : '';
};
function expectations(capture, snapshot) {
  assert.equal(capture.schema, 1, 'capture schema');
  const live = capture.terminals.filter(t => t.state === 'live');
  const panes = new Map(snapshot.panes.map(p => [p.pane_id, p]));
  assert.deepEqual([...panes.keys()].sort(), live.map(t => t.address.replace('/', ':')).sort(), 'missing/extra pane');
  assert.equal(panes.size, snapshot.panes.length, 'duplicate pane IDs');
  const tabs = new Map(snapshot.tabs.map(t => [t.tab_id, t]));
  const spaces = new Map(snapshot.workspaces.map(w => [w.workspace_id, w]));
  assert.equal(tabs.size, snapshot.tabs.length, 'duplicate tabs');
  assert.equal(spaces.size, snapshot.workspaces.length, 'duplicate spaces');
  const membership = new Map();
  for (const [machine, doc] of Object.entries(capture.docs)) {
    for (const sp of doc.value.spaces || []) {
      const members = (sp.tabs || []).flatMap(tab => leaves(tab.root).map(id => ({
        address: machine ? `${machine}/${id}` : id, tab,
      }))).filter(m => live.some(t => t.address === m.address));
      const liveNames = [...new Set(members.map(m => whereName(live.find(t => t.address === m.address))).filter(Boolean))];
      const project = liveNames.length === 1 ? liveNames[0] : (sp.name || '').trim() || path.basename(sp.folder || '') || sp.id;
      for (const m of members) {
        assert(!membership.has(m.address), `duplicate App membership: ${m.address}`);
        membership.set(m.address, { space: sp.id, tab: m.tab.id, tabName: m.tab.name || '', project });
      }
    }
  }
  return live.map(t => {
    const paneId = t.address.replace('/', ':');
    const p = panes.get(paneId), machine = machineOf(t.address), m = membership.get(t.address);
    const w = spaces.get(p.workspace_id), tab = tabs.get(p.tab_id);
    assert(w && tab, `${t.address}: dangling workspace/tab`);
    assert.equal(tab.workspace_id, p.workspace_id, `${t.address}: tab in wrong workspace`);
    assert.equal(p.terminal_id, t.address, `${t.address}: terminal identity changed`);
    const cwd = t.foreground?.cwd || t.cwd || '';
    let project, workspaceId, defaultGroup = false;
    const base = machine ? `machine:${machine}` : 'core';
    if (m) {
      assert.equal(p.workspace_id, `${machine ? `machine:${machine}` : 'core'}:space:${m.space}`, `${t.address}: App space`);
      assert.equal(p.tab_id, `app:${machine ? `${machine}:` : ''}${m.tab}`, `${t.address}: App tab`);
      assert.equal(tab.label, m.tabName, `${t.address}: lost custom App title`);
      project = m.project;
      workspaceId = `${base}:space:${m.space}`;
    } else if (/^[\w.-]{1,64}$/.test(t.labels?.['herdr.space'] || '')) {
      project = whereName(t) || t.labels['herdr.space'];
      workspaceId = `${base}:space:${t.labels['herdr.space']}`;
    } else {
      const clean = path.normalize(cwd).replace(/\/$/, '');
      const isProject = path.isAbsolute(cwd) && /^\/(?:Users|home)\/[^/]+\/.+/.test(clean);
      project = isProject ? path.basename(clean) : machine || 'This computer';
      workspaceId = isProject ? `${base}:cwd:${createHash('sha256').update(clean).digest('hex').slice(0,16)}` : base;
      defaultGroup = !isProject;
    }
    assert.equal(p.workspace_id, workspaceId, `${t.address}: wrong fallback grouping identity`);
    assert.equal(w.label, machine && !defaultGroup ? `${machine} · ${project}` : project, `${t.address}: project name`);
    if (machine) {
      assert(!p.cwd && !w.cwd, `${t.address}: remote path became a local file root`);
      assert.equal(p.tokens?.cwd || '', cwd, `${t.address}: remote display cwd lost`);
    } else assert.equal(p.cwd || '', cwd, `${t.address}: cwd lost`);
    const title = (t.labels?.['pairfob.label'] || t.name || '').trim()
      || (t.status?.app ? customTab(m?.tabName) : '');
    return { paneId, machine, project, workspaceId, title, rawTitle: t.title || '' };
  });
}
module.exports = { expectations };
