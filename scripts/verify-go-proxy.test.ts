import { expect, test } from "bun:test";
import { mkdtemp, rm, writeFile } from "node:fs/promises";
import { tmpdir } from "node:os";
import { join, resolve } from "node:path";

test("Go verification bypasses loopback proxies and preserves both bypass lists", async () => {
  let proxied = 0;
  const origin = Bun.serve({ hostname: "127.0.0.1", port: 0, fetch: () => new Response("loopback reached") });
  const proxy = Bun.serve({ hostname: "127.0.0.1", port: 0, fetch: () => { proxied++; return new Response("proxy refused loopback", { status: 502 }); } });
  const dir = await mkdtemp(join(tmpdir(), "pairfob-go-proxy-"));
  try {
    await writeFile(join(dir, "go"), '#!/bin/sh\n[ "$1" = test ] || exit 2\nprintf "BYPASS=%s\\n" "$no_proxy"\nexec curl --silent --show-error --fail "$PAIRFOB_LOOPBACK_FIXTURE"\n', { mode: 0o755 });
    const proxyURL = `http://127.0.0.1:${proxy.port}`;
    const child = Bun.spawn(["bash", resolve(import.meta.dir, "verify-go.sh"), "--part", "test"], {
      stdout: "pipe", stderr: "pipe",
      env: { ...process.env, PATH: `${dir}:${process.env.PATH}`, PAIRFOB_VERIFY_JOBS: "1",
        PAIRFOB_LOOPBACK_FIXTURE: `http://127.0.0.1:${origin.port}/fixture`,
        http_proxy: proxyURL, https_proxy: proxyURL, HTTP_PROXY: proxyURL, HTTPS_PROXY: proxyURL,
        all_proxy: proxyURL, ALL_PROXY: proxyURL, no_proxy: ".lower.internal", NO_PROXY: ".upper.internal" },
    });
    const [code, stdout, stderr] = await Promise.all([child.exited, new Response(child.stdout).text(), new Response(child.stderr).text()]);
    expect({ code, stderr }).toEqual({ code: 0, stderr: "" });
    expect(stdout).toContain("loopback reached");
    expect(stdout).toContain("localhost,127.0.0.1,::1");
    expect(stdout).toContain(".lower.internal");
    expect(stdout).toContain(".upper.internal");
    expect(proxied).toBe(0);
  } finally {
    origin.stop(true); proxy.stop(true);
    await rm(dir, { recursive: true, force: true });
  }
});
