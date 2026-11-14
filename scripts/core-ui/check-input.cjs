const fs = require('node:fs');
const path = require('node:path');
const assert = require('node:assert/strict');
const names = ['m3','dorm-dev','nerd','mini'];
const root = process.argv[2];
const files = fs.readdirSync(path.join(root,'captures')).filter(f => f.endsWith('.json')).sort();
assert.deepEqual(files,names.map(n => `${n}.json`).sort(),'four expected node captures required');
const commits = new Set(), hosts = new Set();
for (const file of files) {
  const c = JSON.parse(fs.readFileSync(path.join(root,'captures',file)));
  assert.equal(c.schema,1,'capture schema');
  assert.equal(c.node,path.basename(file,'.json'),'capture node/file mismatch');
  assert(c.host && !hosts.has(c.host),'duplicate or missing Core host');
  hosts.add(c.host);
  assert(Number.isFinite(Date.parse(c.capturedAt)),'invalid capture date');
  const age = Date.now()-Date.parse(c.capturedAt);
  assert(age >= -60_000 && age < 30*60_000,'capture over 30 minutes old; recapture all four nodes');
  const commit = /tingxueren\/([a-f0-9]+)@/.exec(c.binary.build)?.[1];
  assert(commit,`${file}: daemon is not identified as our fork`);
  if (process.env.PAIRFOB_EXPECT_BINARY_COMMIT) assert(commit.startsWith(process.env.PAIRFOB_EXPECT_BINARY_COMMIT),`${file}: wrong deployed binary`);
  commits.add(commit);
}
assert.equal(commits.size,1,'deployed daemon revisions differ across nodes');
console.log(`Four captures validated; deployed commit ${[...commits][0]}`);
