import { Server } from "lucide-react";
import { t } from "../../../lib/i18n";
import "./remote-badge.scss";
export function RemoteBadge({ machine }: { machine?: string }) {
  if (!machine) return null;
  const label = t("list.remoteMachine", { machine });
  return <span className="remote-machine-badge" title={label}><Server size={12} aria-hidden="true" /><span>{label}</span></span>;
}
