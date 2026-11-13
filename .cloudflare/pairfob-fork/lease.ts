/** Serialize fork publication, including asset restoration, across build runners. */
import { execFileSync } from "node:child_process";
import { randomUUID } from "node:crypto";
import { resolve } from "node:path";

export const CREATE = `CREATE TABLE IF NOT EXISTS __pairfob_fork_deploy_lease (
  name TEXT PRIMARY KEY, owner TEXT NOT NULL, expires_at INTEGER NOT NULL
)`;
export const acquireSQL = (owner: string): string => `INSERT INTO __pairfob_fork_deploy_lease
  (name, owner, expires_at) VALUES ('publish', '${owner}', unixepoch()+1800)
  ON CONFLICT(name) DO UPDATE SET owner=excluded.owner, expires_at=excluded.expires_at
  WHERE __pairfob_fork_deploy_lease.expires_at < unixepoch() RETURNING owner`;
export const releaseSQL = (owner: string): string =>
  `DELETE FROM __pairfob_fork_deploy_lease WHERE name='publish' AND owner='${owner}'`;

if (import.meta.main) {
  const root = resolve(import.meta.dir, "../..");
  const wrangler = resolve(root, "workers/pairfob-origin/node_modules/.bin/wrangler");
  const config = resolve(import.meta.dir, "wrangler.generated.json");
  const owner = randomUUID();
  const deadline = Date.now() + 20 * 60 * 1000;
  const remaining = (): number => {
    const value = deadline - Date.now();
    if (value <= 0) throw new Error("Fork publication exceeded its 20-minute budget");
    return value;
  };
  function sql(command: string): { results: { owner?: string }[]; success: boolean }[] {
    const output = execFileSync(wrangler, ["d1", "execute", "DB", "--remote", "--config", config, "--json", "--command", command], { cwd: root, encoding: "utf8", timeout: 30000, killSignal: "SIGKILL" });
    const rows = JSON.parse(output);
    if (!Array.isArray(rows) || rows.some(row => !row.success)) throw new Error("Deploy lease D1 query failed");
    return rows;
  }
  sql(CREATE);
  const acquired = sql(acquireSQL(owner)).some(row => row.results.some(item => item.owner === owner));
  if (!acquired) throw new Error("Another fork deployment owns the lease. Retry after it finishes.");
  try {
    // Re-read the live generation AFTER acquiring the lease; the build's earlier
    // pack may have preceded another successful deployment.
    execFileSync(process.execPath, [resolve(import.meta.dir, "prepare.ts")], { cwd: root, stdio: "inherit", timeout: remaining(), killSignal: "SIGKILL" });
    execFileSync(wrangler, ["d1", "migrations", "apply", "DB", "--remote", "--config", config], { cwd: root, stdio: "inherit", timeout: remaining(), killSignal: "SIGKILL" });
    execFileSync(wrangler, ["deploy", "--config", config], { cwd: root, stdio: "inherit", timeout: remaining(), killSignal: "SIGKILL" });
  } finally {
    sql(releaseSQL(owner));
  }
}
