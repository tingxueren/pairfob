import { expect, test } from "bun:test";
import { groupAgents, type AgentCard } from "./ranking";
import { parseListSort, remoteMachine, visibleListAgents } from "./list-policy";
const pane = (id: string, title: string, machine = ""): AgentCard => ({
  paneId: id, workspaceId: machine ? `machine:${machine}:space:${id}` : `core:space:${id}`,
  terminalId: machine ? `${machine}/t1` : "t1", workspaceLabel: machine ? `${machine} · ${title}` : title,
  cwd: "/", agent: "codex", status: "idle",
});
const agents = [pane("z", "Zebra"), pane("r", "Alpha", "nerd"), pane("a", "Alpha"), pane("a2", "Alpha")];
test("fixed names keep local projects first and have stable identity tie breaks", () => {
  const ids = (stamps: Record<string, number>) => groupAgents(agents, "space", stamps, {}, { listSort: "name" }).map(g => g.id);
  expect(ids({ r: 99, z: 50 })).toEqual(["core:space:a", "core:space:a2", "core:space:z", "machine:nerd:space:r"]);
  expect(ids({ a: 999, a2: 1000 })).toEqual(ids({ r: 99, z: 50 }));
});
test("recent groups follow opens and hidden remote pins are excluded without mutating the source", () => {
  expect(groupAgents(agents, "space", { r: 99, z: 50 }).map(g => g.items[0].paneId)).toEqual(["r", "z", "a", "a2"]);
  expect(groupAgents(agents, "space", {}, { r: 1 }, { showRemote: false }).flatMap(g => g.items).map(a => a.paneId)).toEqual(["z", "a", "a2"]);
  expect(agents).toHaveLength(4);
});
test("plain labels and terminal ids cannot hide a local or legacy Herdr workspace", () => {
  expect(remoteMachine({ workspaceId: "w1", terminalId: "nerd/t1" })).toBe("");
  expect(remoteMachine({ workspaceId: "machine:nerd:space:x", terminalId: "t1" })).toBe("");
  expect(remoteMachine(agents[1])).toBe("nerd");
  expect(visibleListAgents(agents)).toBe(agents);
  expect(parseListSort("invalid")).toBe("recent");
});
