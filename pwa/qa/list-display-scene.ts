import { snapshot } from "./data";
/** Mixed local/remote project identity, with a remote attention row. */
export function listDisplaySnapshot() {
  const value = snapshot();
  value.workspaces![0].label = "Zebra";
  value.workspaces![1].label = "Alpha";
  value.workspaces!.push({ workspace_id: "machine:nerd:space:r", label: "nerd · Relay" });
  value.panes!.push({ pane_id: "remote", workspace_id: "machine:nerd:space:r", terminal_id: "nerd/t1",
    label: "Remote task", agent: "codex", agent_status: "blocked" });
  return value;
}
