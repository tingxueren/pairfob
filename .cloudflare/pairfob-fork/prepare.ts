/** Pack the fork with the live immutable assets so old tabs keep loading chunks. */
import { mkdir, readFile, writeFile } from "node:fs/promises";
import { resolve } from "node:path";
import { execFileSync } from "node:child_process";
import { packSelfhostAssets } from "../../scripts/selfhost-pack.ts";

const root = resolve(import.meta.dir, "../..");
const state = resolve(root, ".dev/cloudflare");
const previous = resolve(state, "previous");
const origin = "https://pairfob.tingxueren.com";
const hashed = /^[A-Za-z0-9_.-]+-[A-Za-z0-9_-]{8,}\.(?:js|css|json|wasm|png|svg|webp|woff2?)$/;

type Generation = { id: string; files: string[] };
async function get(path: string): Promise<Response> {
  const response = await fetch(origin + path, { signal: AbortSignal.timeout(20000), redirect: "error" });
  if (!response.ok) throw new Error(`Live asset ${path}: HTTP ${response.status}`);
  return response;
}
function generations(value: unknown): Generation[] {
  const data = value as { schema?: number; generations?: Generation[] };
  if (data?.schema !== 1 || !Array.isArray(data.generations) || !data.generations.length || data.generations.length > 3) {
    throw new Error("Invalid live release inventory");
  }
  for (const row of data.generations) {
    if (!/^[a-f0-9]{16,64}$/.test(row.id) || !Array.isArray(row.files) || row.files.length > 512 || !row.files.every(file => hashed.test(file))) {
      throw new Error("Invalid live asset generation");
    }
  }
  return data.generations;
}
async function liveHistory(): Promise<Generation[]> {
  const response = await fetch(origin + "/assets/releases.json", { signal: AbortSignal.timeout(20000), redirect: "error" });
  if (response.ok) return generations(await response.json());
  if (response.status !== 404) throw new Error(`Live release inventory: HTTP ${response.status}`);
  // First CI deployment follows a manual pack without releases.json.
  const html = await (await get("/pair")).text();
  const match = html.match(/content="\/assets\/(pwa-assets-([a-f0-9]{16})\.json)"/);
  if (!match) throw new Error("Live app has no asset inventory");
  const graph = await (await get("/assets/" + match[1])).json() as { assets?: string[] };
  if (!Array.isArray(graph.assets)) throw new Error("Live asset graph is incomplete");
  return generations({ schema: 1, generations: [{ id: match[2], files: [match[1], ...graph.assets.map(path => {
    if (!path.startsWith("/assets/")) throw new Error("Live asset is outside /assets/");
    return path.slice(8);
  })] }] });
}

await mkdir(resolve(previous, "assets"), { recursive: true });
const history = await liveHistory();
const files = [...new Set(history.flatMap(row => row.files))];
for (let index = 0; index < files.length; index += 8) {
  await Promise.all(files.slice(index, index + 8).map(async file => {
    const bytes = await (await get("/assets/" + file)).arrayBuffer();
    await writeFile(resolve(previous, "assets", file), Buffer.from(bytes));
  }));
}
await writeFile(resolve(previous, "assets/releases.json"), JSON.stringify({ schema: 1, generations: history }));
await packSelfhostAssets({
  pwaDist: resolve(root, "pwa/dist"), dest: resolve(state, "public"),
  installScript: resolve(root, "scripts/install.sh"), previousPublic: previous,
});
const templatePath = resolve(import.meta.dir, "wrangler.json");
const config = JSON.parse(await readFile(templatePath, "utf8"));
// Generated config stays alongside the template; its relative paths remain valid.
const sha = execFileSync("git", ["rev-parse", "--short=12", "HEAD"], { cwd: root, encoding: "utf8" }).trim();
config.vars.BUILD = `tingxueren/${sha}/${new Date().toISOString()}`;
await writeFile(resolve(import.meta.dir, "wrangler.generated.json"), JSON.stringify(config, null, 2) + "\n");
console.log(`Prepared ${config.vars.BUILD}; restored ${files.length} live immutable assets`);
