import { expectSameNode, expectSameNodes } from "../../../../test-support/node-identity";
import { happy, resetBoardTestDOM } from "../../../../test-support/dom";
import { act } from "react";
import { afterEach, beforeEach, describe, expect, mock, test } from "bun:test";
import { lang, setLang } from "../../../lib/i18n";
import type { LiveSession } from "../../../lib/protocol/client";

let visibility: DocumentVisibilityState = "visible";
let baselineLang: ReturnType<typeof lang> = "zh";

let inputListener: (text: string) => void = () => {};
class TestTerminal {
  cols = 80;
  rows = 24;
  modes = { applicationCursorKeysMode: false };
  options: { fontSize?: number; lineHeight?: number; letterSpacing?: number };
  _core = { _renderService: { dimensions: { css: { cell: { width: 8, height: 16 } } } } };
  private root: HTMLElement | null = null;

  constructor(options: { fontSize?: number; lineHeight?: number; letterSpacing?: number }) {
    this.options = { ...options };
  }

  loadAddon(_addon: object): void {}

  open(mount: HTMLElement): void {
    this.root = document.createElement("div");
    this.root.className = "xterm";
    const screen = document.createElement("div");
    screen.className = "xterm-screen";
    screen.append(document.createElement("canvas"));
    const input = document.createElement("textarea");
    input.className = "xterm-helper-textarea";
    this.root.append(screen, input);
    mount.append(this.root);
  }

  resize(cols: number, rows: number): void {
    this.cols = cols;
    this.rows = rows;
  }

  registerLinkProvider(_provider: object): void {}
  onData(listener: (value: string) => void): void { inputListener = listener; }
  onBinary(_listener: (value: string) => void): void {}
  onResize(_listener: () => void): void {}
  input(_value: string): void {}
  focus(): void {}
  reset(): void {}
  write(_data: Uint8Array, done?: () => void): void { done?.(); }
  dispose(): void { this.root?.remove(); }
}

class TestFitAddon {
  fit(): void {}
}

class TestWebglAddon {
  onContextLoss(_listener: () => void): void {}
}

mock.module("./full-terminal-loader", () => ({
  fullTerminalSupported: () => true,
  terminalWebglSupported: () => true,
  loadFullTerminalXterm: async () => ({
    Terminal: TestTerminal,
    FitAddon: TestFitAddon,
    WebglAddon: TestWebglAddon,
  }),
  preloadFullTerminalXterm: () => {},
}));

const { appRoot } = await import("../../../app/dom-root.ts");
const app = appRoot();
const { batch } = await import("../../../shared/model/domain-store.ts");
const { appHost, commitView } = await import("../../../app/host.ts");
const { isAppMounted, mountApp, unmountApp } = await import("../../../app/mount.tsx");
const { registerSessionOwnerPreparer } = await import("../../../app/frame.ts");
const { registerSessionView } = await import("../../../features/session/register.ts");
const { resetTransitionState } = await import("../../../app/transition.ts");
const { setPhase, setNetworkOnline } = await import("../../connection/connection-store.ts");
const { currentScreen, setScreen } = await import("../../../app/navigation-store.ts");
const {
  applyPaneRead, isFullTerminal, openPaneId, resetPaneView, selectPane, setAgentChat, setFullTerminal,
} = await import("../session-store.ts");
const { composeDraft, composeLive, setComposeDraft, setComposeLive } =
  await import("../compose-store.ts");
const { attachLiveSession } = await import("../../computers/catalog-store.ts");
const { applySnapshot } = await import("../../dashboard/catalog-store.ts");
const {
  keysExpanded, paneComposeLive, paneTermMode, setKeysExpanded, setPaneTermMode, termCols,
} = await import("../../settings/preferences-store.ts");
const { openPane } = await import("../../../features/connection/controller.ts");
const { goBackFromPane } = await import("../pane-actions.ts");
const {
  disposeFullTerminal,
  enterFullTerminal,
  handleFullTerminalEvent,
  handleFullTerminalVisibility,
  leaveFullTerminal,
  setFullTerminalComposeLive,
  setTermFit,
} = await import("./full-terminal.ts");
const { resetComposeDrafts } = await import("../drafts/compose-drafts.ts");

const DRAFT = "keep-draft";

function live(): LiveSession {
  return {
    terminalOpen: (_paneId: string, _cols: number, _rows: number) => new Promise(() => undefined),
    terminalClose: async () => undefined,
    terminalInput: async () => undefined,
    terminalResize: async () => undefined,
    terminalScroll: async () => undefined,
    sendText: async () => undefined,
    sendKeys: async () => undefined,
    isConnected: () => true,
    onEvent: () => () => undefined,
    reconnectNow: () => undefined,
    close: () => undefined,
  } as unknown as LiveSession;
}

const DEMO_WORKSPACES = [{ workspace_id: "w1", label: "demo", cwd: "/tmp/demo" }] as const;

function paneCard(paneId: string, status = "idle") {
  return { pane_id: paneId, workspace_id: "w1", agent: "herdr", agent_status: status };
}

// The actual stable App owns composition: typed domain actions publish through
// the installed host's synchronous commitView port. No paint host, no facade
// writes, no manual flush.
function publishDemoPanes(panes: ReadonlyArray<ReturnType<typeof paneCard>>): void {
  applySnapshot({ focused: { pane_id: panes[0]?.pane_id }, workspaces: DEMO_WORKSPACES, panes });
}

function bootFullTerminal(session: LiveSession = live()): void {
  act(() => {
    disposeFullTerminal();
    batch(() => {
      setPhase("live");
      setScreen("pane");
      selectPane("p1");
      resetPaneView();
      applyPaneRead("ready", "h-ready");
      setComposeDraft(DRAFT);
      setComposeLive(false);
      setFullTerminal(true);
      setAgentChat(false);
      setNetworkOnline(true);
      attachLiveSession(session);
      publishDemoPanes([paneCard("p1")]);
      setPaneTermMode("p1", "full");
    });
    commitView();
  });
}

function click(selector: string): void {
  const el = app.querySelector(selector);
  if (!(el instanceof HTMLButtonElement)) throw new Error(`missing ${selector}: ${app.innerHTML.slice(0, 200)}`);
  act(() => el.click());
}

async function waitUntil(predicate: () => boolean, label: string): Promise<void> {
  // The bridge opens across several async hops; a loaded CI runner needs more
  // than the old 500ms budget.
  for (let attempt = 0; attempt < 1000; attempt++) {
    if (predicate()) return;
    await act(async () => { await new Promise<void>((resolve) => window.setTimeout(resolve, 10)); });
  }
  throw new Error(`timed out waiting for ${label}`);
}

beforeEach(async () => {
  await resetBoardTestDOM();
  await (await import("../../../app/connected-loader")).loadConnectedRuntime();
  Object.defineProperty(document, "visibilityState", { configurable: true, get: () => visibility });
  resetTransitionState();
  resetComposeDrafts();
  baselineLang = lang();
  setLang("zh");
  registerSessionOwnerPreparer(registerSessionView);
  // The stable declarative App commits through the host; no legacy renderer.
  act(() => mountApp());
});

afterEach(async () => {
  await act(async () => {
    visibility = "visible";
    await leaveFullTerminal({ rememberGuided: false, paint: false });
    disposeFullTerminal();
    resetComposeDrafts();
    unmountApp();
    setLang(baselineLang);
    registerSessionOwnerPreparer(null);
    attachLiveSession(null);
    resetTransitionState();
    await happy.happyDOM.abort();
  });
  expect(appHost()).toBeNull();
  expect(isAppMounted()).toBeFalse();
});

describe("complete-terminal output recovery", () => {
  test("a forward delta gap reopens automatically without changing the mode or draft", async () => {
    let closes = 0;
    let opens = 0;
    const terminalId = "term_11111111111111111111111111111111";
    const session = live();
    session.terminalOpen = async (paneId, cols, rows) => {
      opens++;
      return {
        operationId: "op_AAECAwQFBgcICQoL",
        terminalId,
        paneId,
        cols,
        rows,
        encoding: "ansi" as const,
      };
    };
    session.terminalClose = async () => { closes++; };
    bootFullTerminal(session);
    await waitUntil(() => opens === 1, "terminal open");

    const terminalFrame = (sequence: string, full: boolean) => ({
      type: "terminal_frame" as const,
      terminalId,
      terminalFrame: {
        terminalId,
        sequence,
        width: 80,
        height: 24,
        full,
        index: 0,
        count: 1,
        data: new Uint8Array([65]),
      },
    });
    act(() => handleFullTerminalEvent(terminalFrame("1", true)));
    act(() => handleFullTerminalEvent(terminalFrame("1", false)));
    act(() => handleFullTerminalEvent(terminalFrame("3", true)));
    act(() => handleFullTerminalEvent(terminalFrame("4", false)));
    act(() => handleFullTerminalEvent(terminalFrame("2", false)));
    await act(async () => { await Promise.resolve(); });
    expect(closes).toBe(0);

    act(() => handleFullTerminalEvent(terminalFrame("6", false)));
    await act(async () => { await Promise.resolve(); });
    expect(closes).toBe(1);
    await waitUntil(() => opens === 2, "automatic frame gap recovery");
    expect(isFullTerminal()).toBeTrue();
    expect(composeDraft()).toBe(DRAFT);
  });


  test("a gap with in-flight text and queued Enter warns without replay", async () => {
    let opens = 0, inputs = 0;
    const terminalId = "term_11111111111111111111111111111111";
    const session = live();
    session.terminalOpen = async (paneId, cols, rows) => { opens++; return {
      operationId: "op_AAECAwQFBgcICQoL", terminalId, paneId, cols, rows, encoding: "ansi" as const,
    }; };
    session.terminalInput = async () => { inputs++; await new Promise(() => undefined); };
    bootFullTerminal(session);
    await waitUntil(() => opens === 1, "terminal open");
    const frame = (sequence: string, full: boolean) => ({ type: "terminal_frame" as const, terminalId,
      terminalFrame: { terminalId, sequence, width: 80, height: 24, full, index: 0, count: 1, data: new Uint8Array([65]) } });
    act(() => handleFullTerminalEvent(frame("1", true)));
    act(() => inputListener("prompt"));
    await waitUntil(() => inputs === 1, "text in flight");
    act(() => inputListener("\r"));
    act(() => handleFullTerminalEvent(frame("3", false)));
    await waitUntil(() => document.querySelector(".full-terminal-state")?.getAttribute("data-stage") === "error", "pending input warning");
    expect(opens).toBe(1);
    expect(inputs).toBe(1);
    expect(document.querySelector(".full-terminal-state-detail")?.textContent).toContain("输入可能未送达");
  });

  test("an exhausted old owner cannot close a newly selected pane", async () => {
    let opens = 0;
    const closed: string[] = [];
    const session = live();
    session.terminalOpen = async (paneId, cols, rows) => ({ operationId: "op_AAECAwQFBgcICQoL",
      terminalId: "term_" + String(++opens).padStart(32, "1"), paneId, cols, rows, encoding: "ansi" as const });
    session.terminalClose = async (id) => { closed.push(id); };
    bootFullTerminal(session);
    await waitUntil(() => opens === 1, "initial open");
    const gap = (id: string) => { const frame = (sequence: string, full: boolean) => ({ type: "terminal_frame" as const,
      terminalId: id, terminalFrame: { terminalId: id, sequence, width: 80, height: 24, full, index: 0, count: 1, data: new Uint8Array([65]) } });
      handleFullTerminalEvent(frame("1", true)); handleFullTerminalEvent(frame("3", false)); };
    act(() => gap("term_" + String(1).padStart(32, "1")));
    await waitUntil(() => opens === 2, "first recovery");
    act(() => gap("term_" + String(2).padStart(32, "1")));
    await waitUntil(() => opens === 3, "second recovery");
    act(() => { gap("term_" + String(3).padStart(32, "1")); selectPane("p2"); });
    await act(async () => { await Promise.resolve(); });
    await waitUntil(() => opens === 4, "new pane bridge");
    expect(closed).toHaveLength(3);
    expect(closed).not.toContain("term_" + String(4).padStart(32, "1"));
    expect(openPaneId()).toBe("p2");
  });
});
