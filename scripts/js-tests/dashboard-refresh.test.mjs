import assert from "node:assert/strict";
import { readFileSync } from "node:fs";
import vm from "node:vm";
import test from "node:test";

function fixture({ focused = false } = {}) {
  let tick;
  let fetches = 0;
  let replacements = 0;
  let focusedAgain = false;
  const token = { value: "old-token" };
  const button = { getAttribute: () => "movie", focus() { focusedAgain = true; } };
  const widget = {
    scrollLeft: 12, scrollTop: 24,
    getAttribute: () => "queue", contains: () => focused,
    querySelector: () => button
  };
  const document = {
    hidden: false, activeElement: focused ? button : {}, addEventListener() {},
    querySelector: selector => selector === "[data-seerr-query]" ? { value: "A completed search remains here" } : null
  };
  const dashboard = {
    queryAll: selector => selector.includes("csrf") ? [token] : [widget],
    replaceNodeContent() { replacements++; }, initWidgets() {}
  };
  const parsed = { querySelector: selector => selector.includes("csrf") ? { value: "new-token" } : {} };
  const window = { VeyraDashboard: dashboard, location: { href: "https://veyra.invalid/dashboard" }, setInterval(fn) { tick = fn; }, setTimeout() {} };
  vm.runInNewContext(readFileSync("web/static/js/dashboard/refresh.js", "utf8"), {
    window, document, DOMParser: class { parseFromString() { return parsed; } },
    fetch: async () => { fetches++; return { ok: true, text: async () => "dashboard" }; }
  });
  dashboard.initAutoRefresh();
  return {
    document, token, widget,
    async refresh() { tick(); await new Promise(resolve => setImmediate(resolve)); },
    state: () => ({ fetches, replacements, focusedAgain })
  };
}

test("completed search leaves widgets refreshing and adopts rotated CSRF", async () => {
  const f = fixture();
  await f.refresh();
  assert.equal(f.state().replacements, 1);
  assert.equal(f.token.value, "new-token");
  assert.equal(f.widget.scrollTop, 24);
});

test("queue refresh preserves filter focus; hidden tabs pause requests", async () => {
  const f = fixture({ focused: true });
  await f.refresh();
  assert.equal(f.state().focusedAgain, true);
  assert.equal(f.state().replacements, 1);
  f.document.hidden = true;
  await f.refresh();
  assert.equal(f.state().fetches, 1);
});

test("calendar times retain the UTC day used by the server", () => {
  const time = { textContent: "23:00 UTC", getAttribute: () => "2026-09-07T23:00:00Z" };
  const dashboard = {
    queryAll: selector => selector.startsWith(".calendar-local-time") ? [time] : [{ getAttribute: () => "2026-09-07", classList: { toggle() {} } }],
    registerWidget(fn) { fn(); }
  };
  vm.runInNewContext(readFileSync("web/static/js/dashboard/calendar.js", "utf8"), { window: { VeyraDashboard: dashboard }, Date: class extends Date { toLocaleTimeString() { return "11:00 NZST"; } } });
  assert.equal(time.textContent, "23:00 UTC");
});
