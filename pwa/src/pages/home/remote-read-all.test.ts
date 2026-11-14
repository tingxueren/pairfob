import { afterEach, expect, test } from "bun:test";
import "../../../test-support/boot-dom";
import { applySnapshot, dashboardStore, resetDashboard } from "../../features/dashboard/catalog-store";
import { resetHerdPresentationChoices, setShowRemote } from "../../features/settings/preferences-store";
import { herdActionPorts } from "./herd-bridge";
import { visibleListAgents } from "../../lib/list-policy";
afterEach(() => { resetDashboard(); resetHerdPresentationChoices(); localStorage.removeItem("pairfob:showRemote"); });
test("mark all read leaves hidden remote completions unread and undo restores only visible completions", () => {
  resetDashboard(); resetHerdPresentationChoices();
  applySnapshot({ workspaces: [{ workspace_id: "core:space:a", label: "a" }, { workspace_id: "machine:nerd:space:b", label: "nerd · b" }],
    panes: [
      { pane_id: "local", workspace_id: "core:space:a", terminal_id: "t1", agent: "codex", agent_status: "done", completion_id: "local-turn" },
      { pane_id: "remote", workspace_id: "machine:nerd:space:b", terminal_id: "nerd/t1", agent: "codex", agent_status: "done", completion_id: "remote-turn" },
    ] });
  setShowRemote(false);
  const receipt = herdActionPorts().markAllRead();
  expect(receipt?.count).toBe(1);
  setShowRemote(true);
  expect(visibleListAgents(dashboardStore.get().agents).map(a => [a.paneId, a.status])).toEqual([["local", "idle"], ["remote", "done"]]);
  receipt?.undo();
  expect(dashboardStore.get().agents.map(a => a.status)).toEqual(["done", "done"]);
});
