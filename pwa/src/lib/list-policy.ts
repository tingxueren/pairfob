/** Device-local list presentation; runtime identity and live sessions stay unchanged. */
type ListEntry = { workspaceId?: string; terminalId?: string };
export type ListSort = "recent" | "name";
export type ListPolicy = { listSort?: ListSort; showRemote?: boolean };
export function parseListSort(raw: string | null | undefined): ListSort {
  return raw === "name" ? "name" : "recent";
}
/** Only the core adapter's machine-qualified workspace identity marks a remote. */
export function remoteMachine(agent: ListEntry): string {
  const match = /^machine:([^:]+)(?::space:.+)?$/.exec(agent.workspaceId ?? "");
  if (!match || !agent.terminalId?.startsWith(match[1] + "/")) return "";
  return match[1];
}
export function visibleListAgents<T extends ListEntry>(agents: readonly T[], policy: ListPolicy = {}): readonly T[] {
  return policy.showRemote === false ? agents.filter(agent => !remoteMachine(agent)) : agents;
}
export function projectName(title: string, machine: string): string {
  const prefix = machine + " · ";
  return machine && title.startsWith(prefix) ? title.slice(prefix.length) : title;
}
const names = new Intl.Collator("zh-CN", { numeric: true, sensitivity: "base" });
export function compareProjectNames(a: string, b: string): number {
  return names.compare(a, b) || (a < b ? -1 : a > b ? 1 : 0);
}
