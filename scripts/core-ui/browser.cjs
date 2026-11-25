// Replay real-node metadata in the production React App. QA blocks all live
// transport: these checks cannot type in, resize or close a user's terminal.
const fs = require('node:fs');
const path = require('node:path');
const assert = require('node:assert/strict');
const cp = require('node:child_process');
const { expectations } = require('./oracle.cjs');
const { sourceDigest } = require('./source-digest.cjs');
const repo = path.resolve(__dirname, '../..');
const modulePath = process.env.PAIRFOB_PLAYWRIGHT_MODULE || path.join(repo, '.dev/browser-tools/node_modules/playwright');
const { chromium } = require(modulePath);
const root = path.resolve(process.argv[2]);
const base = process.env.PAIRFOB_CORE_UI_URL;
if (!base || !['localhost', '127.0.0.1'].includes(new URL(base).hostname)) throw new Error('QA URL must be loopback');
const surfaces = [['phone',390,844,true], ['tablet820',820,768,true], ['tablet1024',1024,768,true], ['desktop',1440,900,false]];
const report = { schema: 1, source: cp.execFileSync('git', ['rev-parse', 'HEAD'], {cwd:repo,encoding:'utf8'}).trim(),
  sourceDigest: sourceDigest(), capturedInput: root, browser: require(path.join(modulePath, 'package.json')).version,
  boundary: 'real Core reads -> production Observe + async doc loading -> production PWA stores/React DOM; no relay, running daemon cache or physical phone',
  cases: [], failures: [], coverage: { nodes: 0, customNames: 0, remotePanes: 0, cwdFallbacks: 0 } };
async function settle(page) {
  await page.evaluate(() => new Promise(resolve => requestAnimationFrame(() => requestAnimationFrame(resolve))));
}
async function inject(page, snapshot, capture) {
  await page.evaluate(async ({snapshot,host}) => {
    const loaded = p => import(performance.getEntriesByType('resource').find(e => new URL(e.name).pathname === p)?.name || p);
    const [{replaceAgentsFromSnapshot}, prefs, {commitView}, computers, runtime] = await Promise.all([
      loaded('/src/features/dashboard/catalog-store.ts'), loaded('/src/features/settings/preferences-store.ts'), loaded('/src/app/host.ts'),
      loaded('/src/features/computers/catalog-store.ts'), loaded('/src/features/connection/runtime-store.ts'),
    ]);
    const pair = {...computers.credential(),hostname:host};
    computers.setCredential(pair);
    computers.setComputers([pair]);
    // GetConfig normalizes a Core descriptor to the frozen wire enum "herdr".
    runtime.applyRuntimeIdentity({herdHost:host,runtimeKind:"herdr"});
    prefs.resetHerdPresentationChoices();
    prefs.setListGroup('space');
    prefs.setListSort('name');
    prefs.setListGroupCollapsed(Object.fromEntries(snapshot.workspaces.map(w => [w.workspace_id, false])));
    replaceAgentsFromSnapshot(snapshot);
    commitView();
  }, {snapshot,host:capture.host});
  await settle(page);
}
async function dom(page) {
  return page.evaluate(() => [...document.querySelectorAll('.herd-group:not(.is-leaving)')].map(g => ({
    id: g.querySelector('.group-title').getAttribute('data-trigger-of'),
    name: g.querySelector('.group-name').textContent,
    remote: g.querySelector('.group-title .remote-machine-badge')?.textContent || '',
    rows: [...g.querySelectorAll('[data-pane-id]')].map(r => ({ id: r.dataset.paneId,
      name: r.querySelector('.card-name')?.textContent || '', status: r.querySelector('.card-status')?.className || '', remote: r.querySelector('.remote-machine-badge')?.textContent || '' })),
  })));
}
function checkDOM(groups, expected) {
  const rows = groups.flatMap(g => g.rows.map(r => ({...r, group:g})));
  assert.deepEqual(rows.map(r => r.id).sort(), expected.map(e => e.paneId).sort(), 'DOM lost/duplicated panes');
  for (const e of expected) {
    const row = rows.find(r => r.id === e.paneId);
    assert.equal(row.group.id, e.workspaceId, `${e.paneId}: DOM workspace`);
    assert.equal(row.group.name, e.project, `${e.paneId}: DOM project title`);
    if (e.status) assert(row.status.split(/\s+/).includes('is-' + e.status),e.paneId + ': DOM lost agent status ' + e.status + ': ' + row.status);
    assert(row.name.trim(), `${e.paneId}: blank DOM title`);
    if (e.title) assert.equal(row.name, e.title, `${e.paneId}: DOM custom title`);
    assert.equal(row.remote, e.machine ? `远端 · ${e.machine}` : '', `${e.paneId}: remote row badge`);
    assert.equal(row.group.remote, row.remote, `${e.paneId}: remote group badge`);
  }
}
async function choose(page, title, touch) {
  await page.evaluate(async () => {
    const p = '/src/features/dashboard/components/group-mode-sheet.tsx';
    const url = performance.getEntriesByType('resource').find(e => new URL(e.name).pathname === p)?.name || p;
    (await import(url)).openGroupModeSheet();
  });
  const choice = page.getByText(title, {exact:true});
  await choice.waitFor();
  if (touch) await choice.tap(); else await choice.click();
  await settle(page);
}
(async () => {
  fs.mkdirSync(path.join(root, 'screenshots'), {recursive:true,mode:0o700});
  const browser = await chromium.launch({headless:true,args:['--use-angle=swiftshader','--enable-unsafe-swiftshader']});
  try {
    const files = fs.readdirSync(path.join(root, 'captures')).filter(f => f.endsWith('.json')).sort();
    assert(files.length > 0, 'no real captures');
    for (const file of files) {
      const capture = JSON.parse(fs.readFileSync(path.join(root, 'captures', file)));
      const snapshot = JSON.parse(fs.readFileSync(path.join(root, 'projected', file)));
      const expected = expectations(capture, snapshot);
      assert(expected.length > 0, `${file}: no live panes; cannot accept an empty node`);
      report.coverage.nodes++;
      report.coverage.customNames += expected.filter(e => e.title).length;
      report.coverage.remotePanes += expected.filter(e => e.machine).length;
      report.coverage.cwdFallbacks += expected.filter(e => e.workspaceId.includes(':cwd:')).length;
      for (const [surface,width,height,touch] of surfaces) {
        const name = `${path.basename(file,'.json')}-${surface}`;
        const context = await browser.newContext({viewport:{width,height},hasTouch:touch,isMobile:surface==='phone',locale:'zh-CN',timezoneId:'UTC'});
        const page = await context.newPage();
        const errors = [];
        page.on('pageerror', e => errors.push(e.message));
        try {
          await page.goto(`${base}/qa/index.html?scene=home-grouped&lang=zh`);
          await page.waitForFunction(() => window.qa);
          await page.evaluate(() => window.qa.ready);
          await inject(page, snapshot, capture);
          const before = await dom(page);
          await page.screenshot({path:path.join(root,'screenshots',`${name}.png`),fullPage:true});
          checkDOM(before, expected);
          // An App rename arriving in a later snapshot must repaint this same
          // page; this specifically guards the "old name until reload" report.
          const renamed = expected.find(e => e.title && snapshot.tabs.some(tab => tab.label === e.title)
            && !snapshot.panes.find(p => p.pane_id === e.paneId).label);
          if (renamed) {
            const changed = structuredClone(snapshot);
            const pane = changed.panes.find(p => p.pane_id === renamed.paneId);
            const title = renamed.title + ' [QA rename]';
            changed.tabs.find(tab => tab.tab_id === pane.tab_id).label = title;
            async function update(value) {
              const result = await page.evaluate(async value => {
                const loaded = p => import(performance.getEntriesByType('resource').find(e => new URL(e.name).pathname === p)?.name || p);
                const [{applySnapshot},{commitView}] = await Promise.all([loaded('/src/features/dashboard/catalog-store.ts'),loaded('/src/app/host.ts')]);
                const result = applySnapshot(value);
                commitView();
                return result.unchanged;
              }, value);
              assert.equal(result,false,'rename snapshot incorrectly treated as unchanged');
              await settle(page);
            }
            await update(changed);
            assert.equal((await dom(page)).flatMap(g => g.rows).find(r => r.id === renamed.paneId).name,title,'live title refresh');
            await update(snapshot);
            checkDOM(await dom(page),expected);
          }
          const collator = new Intl.Collator('zh-CN',{numeric:true,sensitivity:'base'});
          const compareNames = (a,b) => collator.compare(a,b) || (a < b ? -1 : a > b ? 1 : 0);
          // Keep the unbound bucket last; machine, project and identity use the
          // production list policy's deterministic tie-break for equal collation.
          const sorted = [...before].sort((a,b) => Number(a.id==='unbound')-Number(b.id==='unbound')
            || Number(!!a.remote)-Number(!!b.remote) || compareNames(a.remote,b.remote)
            || compareNames(a.name,b.name) || compareNames(a.id,b.id));
          assert.deepEqual(before.map(g => g.id), sorted.map(g => g.id), 'project name order');
          // A recent-open stamp must not disturb fixed name order.
          const last = before.at(-1).rows[0].id;
          await page.evaluate(async id => {
            const p = '/src/features/settings/preferences-store.ts';
            const url = performance.getEntriesByType('resource').find(e => new URL(e.name).pathname === p)?.name || p;
            (await import(url)).rememberPane(id);
          }, last);
          await settle(page);
          assert.deepEqual((await dom(page)).map(g => g.id),before.map(g => g.id),'fixed order moved after recent open');
          await choose(page,'最近打开',touch);
          assert.equal((await dom(page))[0].id,before.at(-1).id,'recent order ignored last open');
          await choose(page,'项目名称',touch);
          await choose(page,'显示远端会话',touch);
          const local = await dom(page);
          checkDOM(local, expected.filter(e => !e.machine));
          assert.equal(await page.evaluate(() => localStorage.getItem('pairfob:showRemote')), '0', 'remote choice not persisted');
          await choose(page,'显示远端会话',touch);
          checkDOM(await dom(page), expected);
          const qa = await page.evaluate(() => window.qa.snapshot());
          assert(!qa.overflow.documentX && !qa.overflow.appX, 'horizontal overflow');
          assert.deepEqual(errors, [], 'browser errors');
          assert.deepEqual(qa.errors, [], 'QA errors');
          report.cases.push({name,passed:true,titleRefresh:!!renamed,panes:expected.length,groups:before.length,rows:before,binary:capture.binary});
          console.log(`PASS ${name}: ${expected.length} panes / ${before.length} projects`);
        } catch (error) {
          await page.screenshot({path:path.join(root,'screenshots',`${name}-failed.png`),fullPage:true}).catch(()=>{});
          report.failures.push({name,message:error.stack});
          console.error(`FAIL ${name}: ${error.message}`);
        } finally { await context.close(); }
      }
    }
  } finally { await browser.close(); }
  assert.equal(sourceDigest(),process.env.PAIRFOB_CORE_UI_SOURCE,'source changed during acceptance; rerun');
  assert(report.coverage.remotePanes > 0, 'remote coverage missing; include a Core with linked nodes');
  assert(report.coverage.customNames > 0, 'custom App title coverage missing');
})().catch(error => report.failures.push({name:'suite',message:error.stack})).finally(() => {
  report.passed = report.failures.length === 0;
  fs.writeFileSync(path.join(root,'report.json'), JSON.stringify(report,null,2)+'\n', {mode:0o600});
  console.log(`Report: ${path.join(root,'report.json')}`);
  if (!report.passed) process.exitCode = 1;
});
