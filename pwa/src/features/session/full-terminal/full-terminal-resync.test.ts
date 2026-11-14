import { expect, test } from "bun:test";
import { FullTerminalResync } from "./full-terminal-resync";

test("a flapping view gets two recoveries per owner in thirty seconds", () => {
  let now = 0;
  const recovery = new FullTerminalResync(() => now);
  expect(recovery.admit("phone/p1")).toBeTrue();
  now = 29_999;
  expect(recovery.admit("phone/p1")).toBeTrue();
  expect(recovery.admit("phone/p1")).toBeFalse();
  now = 30_000;
  expect(recovery.admit("phone/p1")).toBeTrue();
  expect(recovery.admit("phone/p2")).toBeTrue();
});


test("a retired owner does not reopen after its close finishes", async () => {
  const recovery = new FullTerminalResync();
  let finish!: () => void;
  const close = new Promise<void>((resolve) => { finish = resolve; });
  let current = true, opens = 0;
  const task = recovery.recover({ owner: "phone/p1", pause: () => close,
    current: () => current, resume: async () => { opens++; } });
  current = false;
  finish();
  expect(await task).toBeTrue();
  expect(opens).toBe(0);
});

test("view recovery waits for retirement and does not run a blocked retry", async () => {
  const recovery = new FullTerminalResync();
  const order: string[] = [];
  expect(await recovery.recover({ owner: "phone/p1", pause: async () => { order.push("close"); },
    current: () => true, resume: async () => { order.push("open"); } })).toBeTrue();
  expect(order).toEqual(["close", "open"]);
  recovery.retry(true, () => order.push("retry"));
  expect(order).toEqual(["close", "open"]);
});
