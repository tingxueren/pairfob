import { copy, type LocalizedText } from "../../../lib/i18n";
import { messageCopy } from "../../../lib/notices";
import { WEBGL_CONTEXT_LOST, WEBGL_UNAVAILABLE } from "./full-terminal-renderer";

export function webglFailure(error: unknown): LocalizedText {
  const message = error instanceof Error ? error.message : "";
  if (message === WEBGL_CONTEXT_LOST) return copy("ft.webglLost");
  if (message === WEBGL_UNAVAILABLE) return copy("ft.webglUnavailable");
  return messageCopy(error);
}

/** A lost graphics context retires its renderer after asking the bridge to close. */
export function rendererContextLost(active: boolean, suspend: (reason: LocalizedText) => void, dispose: () => void, fail: (reason: LocalizedText) => void): void {
  if (!active) return;
  const reason = copy("ft.loadFail", { error: copy("ft.webglLost") });
  suspend(reason);
  dispose();
  fail(reason);
}
