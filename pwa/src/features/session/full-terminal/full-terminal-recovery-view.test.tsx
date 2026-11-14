import { afterEach, expect, test } from "bun:test";
import { act, createElement } from "react";
import { resetBoardTestDOM } from "../../../../test-support/dom";
import { renderReact, unmountReact } from "../../../../test-support/react-harness";
import { appRoot } from "../../../app/dom-root";
import { FullTerminalStateLayer } from "./full-terminal-state-layer";
import { publishFullTerminalView, resetFullTerminalView } from "./full-terminal-view";
afterEach(() => { unmountReact(); resetFullTerminalView(); });
test("a foreground reopen keeps the last screen visible while a fresh open remains covered", async () => {
  await resetBoardTestDOM();
  renderReact(createElement(FullTerminalStateLayer, { onRetry() {} }));
  const base = { owner: "one", paneId: "p1", title: "terminal", working: false, retry: false,
    busy: true, composeLive: false, keyboardOpen: false, detail: "正在打开终端…" };
  act(() => publishFullTerminalView({ ...base, stage: "opening" }));
  expect(appRoot().querySelector(".full-terminal-state.is-retained")).toBeNull();
  act(() => publishFullTerminalView({ ...base, stage: "opening", hasFrame: true }));
  expect(appRoot().querySelector(".full-terminal-state.is-retained")).not.toBeNull();
  act(() => publishFullTerminalView({ ...base, stage: "error", hasFrame: true, retry: true }));
  expect(appRoot().querySelector(".full-terminal-state.is-retained")).toBeNull();
});
