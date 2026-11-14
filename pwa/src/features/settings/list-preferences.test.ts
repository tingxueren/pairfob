import { afterEach, expect, test } from "bun:test";
import "../../../test-support/boot-dom";
import { hydratePreferences, initialPreferences, listPolicy, preferencesStore, resetHerdPresentationChoices,
  setListSort, setShowRemote } from "./preferences-store";
import { herdListScope, heldShownListOrder, rememberShownListOrder } from "../dashboard/list-order";
import { herdPreferencesKey } from "../../pages/home/herd-bridge";
afterEach(() => { resetHerdPresentationChoices(); localStorage.removeItem("pairfob:listSort"); localStorage.removeItem("pairfob:showRemote"); });
test("display preferences persist, hydrate, and invalidate held list ordering", () => {
  resetHerdPresentationChoices();
  const sig = herdPreferencesKey(preferencesStore.get());
  const scope = herdListScope();
  rememberShownListOrder({ p1: 10 }, ["p1"]);
  setListSort("name");
  setShowRemote(false);
  expect(localStorage.getItem("pairfob:listSort")).toBe("name");
  expect(localStorage.getItem("pairfob:showRemote")).toBe("0");
  expect(herdPreferencesKey(preferencesStore.get())).not.toBe(sig);
  expect(herdListScope()).not.toBe(scope);
  expect(heldShownListOrder({ p1: 99 }, ["p1"])).toEqual({ p1: 99 });
  resetHerdPresentationChoices();
  hydratePreferences({ read: key => localStorage.getItem(key), desk: false });
  expect(listPolicy()).toEqual({ listSort: "name", showRemote: false });
});
test("missing and invalid stored display preferences preserve defaults", () => {
  hydratePreferences({ read: () => "invalid", desk: false });
  expect(listPolicy()).toEqual({ listSort: "recent", showRemote: true });
  expect(initialPreferences().showRemote).toBeTrue();
});
