/** Run on QA Vite with a fresh context at 390/820/1024 touch and 1440 mouse widths. */
(async function checkListDisplay(page, { touch = false, name = "phone", output = "/tmp" } = {}) {
  const p = page;
  const profile = { touch, name };
  const url = new URL("/qa/index.html?scene=home-list-display&lang=zh", page.url()).href;
  const assert = {
    equal(a, b) { if (a !== b) throw new Error(JSON.stringify({ a, b })); },
    deepEqual(a, b) { if (JSON.stringify(a) !== JSON.stringify(b)) throw new Error(JSON.stringify({ a, b })); },
    ok(a) { if (!a) throw new Error("Browser assertion failed"); },
  };
const press=async loc=>profile.touch?loc.tap():loc.click();
await p.goto(url);await p.waitForFunction(()=>window.qa);await p.evaluate(()=>window.qa.ready);
const menu=p.getByRole('button',{name:'分组方式：工作区',exact:true});await press(menu);
await press(p.getByText('项目名称',{exact:true}));
await p.waitForTimeout(220);
const titles=await p.locator('.group-name').allTextContents();assert.deepEqual(titles,['置顶','Alpha','Zebra','Relay']);
assert.ok((await p.locator('.remote-machine-badge').allTextContents()).every(t=>t==='远端 · nerd'));
await press(menu);await p.screenshot({path:output + '/list-menu-'+profile.name+'.png'});await press(p.getByText('显示远端会话',{exact:true}));await p.waitForTimeout(250);
assert.equal(await p.locator('[data-pane-id="remote"]').count(),0);
assert.equal(await p.locator('.remote-machine-badge:visible').count(),0);
assert.equal(await p.evaluate(()=>localStorage.getItem('pairfob:showRemote')),'0');
await press(menu);assert.equal(await p.getByText('项目名称',{exact:true}).locator('..').locator('..').getAttribute('aria-current'),'true');
await press(p.getByText('显示远端会话',{exact:true}));await p.waitForTimeout(250);
assert.equal(await p.locator('.group-name').filter({hasText:'Relay'}).count(),1);
if(!profile.touch){await menu.focus();await p.keyboard.press('Enter');await p.getByText('最近打开',{exact:true}).waitFor();await p.keyboard.press('Escape');}
await p.screenshot({path:output + '/list-groups-'+profile.name+'.png'});
assert.equal(await p.evaluate(()=>document.documentElement.scrollWidth>innerWidth+1),false);
await p.goto(new URL('/qa/index.html?scene=terminal-frame&lang=zh', url).href);await p.waitForFunction(()=>window.qa);await p.evaluate(()=>window.qa.ready);
await p.waitForFunction(()=>document.querySelector('canvas'));
const before=await p.locator('.full-terminal-host').boundingBox();
await p.evaluate(()=>{window.qa.hold('terminalOpen');window.qa.emit({type:'reconnecting'});window.qa.emit({type:'connected'});});
await p.waitForFunction(()=>document.querySelector('.full-terminal-state.is-retained'));
const state=await p.locator('.full-terminal-state').boundingBox();const after=await p.locator('.full-terminal-host').boundingBox();assert.deepEqual(after,before);assert.ok(state.height<100);assert.ok(state.width<before.width);
await p.screenshot({path:output + '/terminal-recovery-'+profile.name+'.png'});
await p.evaluate(()=>window.qa.release('terminalOpen'));await p.waitForFunction(()=>document.querySelector('.full-terminal-state').hidden);

  return { profile: name, titles, notice: state, geometryUnchanged: true };
})
