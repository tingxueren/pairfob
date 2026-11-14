// Match actual tracked + nonignored source bytes, including uncommitted edits.
const cp = require('node:child_process');
const fs = require('node:fs');
const crypto = require('node:crypto');
function sourceDigest() {
  const files = [...new Set(cp.execFileSync('git', ['ls-files','-z','--cached','--others','--exclude-standard'])
    .toString().split('\0').filter(Boolean))].sort();
  const hash = crypto.createHash('sha256');
  for (const file of files) {
    hash.update(file + '\0');
    if (!fs.existsSync(file)) { hash.update('deleted\0'); continue; }
    const info = fs.lstatSync(file);
    hash.update(`${info.mode}\0`);
    hash.update(info.isSymbolicLink() ? fs.readlinkSync(file) : fs.readFileSync(file));
    hash.update('\0');
  }
  return hash.digest('hex');
}
module.exports = { sourceDigest };
if (require.main === module) console.log(sourceDigest());
