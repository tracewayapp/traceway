import assert from "node:assert/strict";
import { mkdir } from "node:fs/promises";
import { resolve } from "node:path";
import { chromium } from "../shop/traffic/node_modules/playwright/index.mjs";
const output = resolve(process.argv[2] ?? "../triggersequence");
await mkdir(output, { recursive: true });
const apiBase = "http://localhost:8082/api",
  dashboard = "http://localhost:5180",
  fixture = "http://localhost:5190";
const auth = await fetch(`${apiBase}/login`, {
  method: "POST",
  headers: { "Content-Type": "application/json" },
  body: JSON.stringify({ email: "admin@localhost.com", password: "admin" }),
});
assert(auth.ok);
const { token } = await auth.json();
async function api(path, body, method = body ? "POST" : "GET") {
  const response = await fetch(`${apiBase}${path}`, {
    method,
    headers: {
      Authorization: `Bearer ${token}`,
      "Content-Type": "application/json",
    },
    body: body ? JSON.stringify(body) : undefined,
  });
  const text = await response.text();
  assert(response.ok, `${path}: ${response.status} ${text}`);
  return text ? JSON.parse(text) : null;
}
const project = (await api("/projects")).find(
  (p) => p.name === "Shoply Storefront",
);
assert(project);
const scope = `projectId=${project.id}`;
const currentRules = await api(`/sdk-config/rules?${scope}`);
for (const r of currentRules.data)
  if (r.name === "User journey replay")
    await api(`/sdk-config/rules/${r.id}?${scope}`, undefined, "DELETE");
const b = await chromium.launch({ headless: true });
const admin = await b.newPage({
  viewport: { width: 1440, height: 1100 },
  timezoneId: "UTC",
});
const errors = [];
admin.on("pageerror", (e) => errors.push(e.message));
const shot = async (name, page = admin) => {
  await page.waitForTimeout(600);
  await page.screenshot({ path: resolve(output, name), fullPage: true });
  console.log("Screenshot", name);
};
const search = async (triggerId = "") =>
  api(`/sessions?${scope}`, {
    fromDate: new Date(Date.now() - 3600000).toISOString(),
    toDate: new Date(Date.now() + 60000).toISOString(),
    pagination: { page: 1, pageSize: 50 },
    orderBy: "started_at",
    sortDirection: "desc",
    triggerId,
  });
async function until(fn, label) {
  const end = Date.now() + 30000;
  let value;
  while (Date.now() < end) {
    value = await fn();
    if (value) return value;
    await new Promise((r) => setTimeout(r, 500));
  }
  throw Error(`Timed out: ${label}`);
}
try {
  await admin.goto(`${dashboard}/login`);
  await admin.locator("#email").fill("admin@localhost.com");
  await admin.locator("#password").fill("admin");
  await admin.getByRole("button", { name: "Login", exact: true }).click();
  await admin.waitForURL((u) => u.pathname != "/login");
  await admin.goto(`${dashboard}/data-collection?${scope}`);
  await admin.getByRole("button", { name: "New Rule", exact: true }).click();
  await admin.locator("#sdk-rule-name").fill("User journey replay");
  const definitions = [
    [
      "Open user journey",
      [
        ["click", "#open-user"],
        ["navigation", "/users/{id}"],
        ["log", "User loaded"],
      ],
    ],
    [
      "Profile inspection",
      [
        ["hover", "#hint"],
        ["log", "User loaded"],
      ],
    ],
    [
      "Full page journey",
      [
        ["click", "#full-navigation"],
        ["navigation", "/users/{id}"],
        ["log", "User loaded"],
      ],
    ],
  ];
  for (let n = 0; n < definitions.length; n++) {
    await admin
      .getByRole("button", { name: "Add recording trigger", exact: true })
      .click();
    const card = admin.getByTestId("sequence-trigger").nth(n);
    await card
      .getByLabel("Trigger name", { exact: true })
      .fill(definitions[n][0]);
    for (let i = 0; i < definitions[n][1].length; i++) {
      if (i)
        await card
          .getByRole("button", { name: "Add step", exact: true })
          .click();
      const step = card.getByTestId("sequence-step").nth(i);
      const [kind, value] = definitions[n][1][i];
      await step.getByLabel("Event", { exact: true }).selectOption(kind);
      await step.getByRole("textbox").fill(value);
    }
    if (n === 0) {
      await card
        .getByRole("button", { name: "Move step 3 up", exact: true })
        .click();
      assert.equal(
        await card
          .getByTestId("sequence-step")
          .nth(1)
          .getByRole("textbox")
          .inputValue(),
        "User loaded",
      );
      await card
        .getByRole("button", { name: "Move step 2 down", exact: true })
        .click();
      await card.getByRole("button", { name: "Add step", exact: true }).click();
      await card
        .getByRole("button", { name: "Remove step 4", exact: true })
        .click();
      await admin.setViewportSize({ width: 1440, height: 1700 });
      await card.scrollIntoViewIfNeeded();
      await admin.waitForTimeout(300);
      await card.screenshot({
        path: resolve(output, "01-sequence-builder.png"),
      });
      await admin.setViewportSize({ width: 1440, height: 1100 });
    }
  }
  await admin
    .getByRole("button", { name: "New Rule", exact: true })
    .last()
    .click();
  await admin.getByRole("alertdialog").waitFor({ state: "detached" });
  const ruleResponse = await api(`/sdk-config/rules?${scope}`);
  const rules = ruleResponse.data;
  const rule = rules.find((r) => r.name === "User journey replay");
  assert(rule);
  const triggers = rule.settings.triggers;
  assert.equal(triggers.length, 3);
  console.log("PASS visual builder saved three sequences without JSON editing");
  // Editing and saving through the form must preserve the trigger IDs and matchers.
  await admin.getByRole("button", { name: "Edit rule" }).first().click();
  await admin.getByTestId("sequence-trigger").first().waitFor();
  assert.equal(await admin.getByTestId("sequence-trigger").count(), 3);
  await admin.getByRole("button", { name: "Update Rule", exact: true }).click();
  await admin.getByRole("alertdialog").waitFor({ state: "detached" });
  const user = await b.newPage({ viewport: { width: 1200, height: 900 } });
  await user.goto(fixture);
  await user.evaluate(() => window.ready);
  await user.waitForTimeout(1800);
  await user.getByRole("button", { name: "Confirm user loaded" }).click();
  await user.evaluate(() => window.sdk.flush());
  await user.waitForTimeout(1000);
  assert.equal((await search(triggers[0].id)).pagination.total, 0);
  await user.locator("#open-user span").click();
  await user.getByRole("button", { name: "Confirm user loaded" }).click();
  await user.evaluate(() => window.sdk.flush());
  const first = await until(async () => {
    const r = await search(triggers[0].id);
    return r.data[0]?.recordingReasons?.length === 1 && r.data[0];
  }, "click/navigation/log reason");
  assert(
    first.recordingReasons[0].sequence.includes("navigation(/users/{id})"),
  );
  console.log(
    "PASS ordered click/navigation/log, nested click target, negative out-of-order log",
  );
  await user.locator("#hint").hover();
  await user.getByRole("button", { name: "Confirm user loaded" }).click();
  await user.evaluate(() => window.sdk.flush());
  await until(async () => {
    const r = await search(triggers[1].id);
    return r.data[0]?.recordingReasons?.length === 2;
  }, "two reasons on one session");
  await user.locator("#hint").hover();
  await user.getByRole("button", { name: "Confirm user loaded" }).click();
  await user.evaluate(() => window.sdk.flush());
  assert.equal((await search(triggers[1].id)).pagination.total, 1);
  console.log(
    "PASS hover/log and multiple deduplicated trigger reasons on one session",
  );
  const reload = await b.newPage();
  await reload.goto(fixture);
  await reload.evaluate(() => window.ready);
  await reload.waitForTimeout(1200);
  await reload.locator("#full-navigation").click();
  await reload.waitForURL("**/users/42");
  await reload.evaluate(() => window.ready);
  await reload.waitForTimeout(1200);
  await reload.getByRole("button", { name: "Confirm user loaded" }).click();
  await reload.evaluate(() => window.sdk.flush());
  await until(
    async () => (await search(triggers[2].id)).pagination.total === 1,
    "full-page sequence restoration",
  );
  console.log("PASS pending sequence survives same-tab full-page navigation");
  // Close each capture through the normal flush path, then verify a playable segment was stored.
  await user.evaluate(() => window.sdk.flush());
  await reload.evaluate(() => window.sdk.flush());
  await until(async () => {
    const r = await search(triggers[0].id);
    return r.data[0]?.hasRecording;
  }, "replay arrived");
  await admin.goto(`${dashboard}/sessions?${scope}&preset=24h`);
  await admin
    .getByText(
      "click(#open-user) → navigation(/users/{id}) → log(User loaded)",
      { exact: true },
    )
    .waitFor();
  await shot("02-session-trigger-tags.png");
  await admin
    .getByLabel("Recording trigger", { exact: true })
    .selectOption(triggers[1].id);
  await admin.waitForTimeout(1000);
  assert.equal(
    new URL(admin.url()).searchParams.get("triggerId"),
    triggers[1].id,
  );
  await shot("03-trigger-filter.png");
  await admin.goto(
    `${dashboard}/sessions/${first.id}?${scope}&startedAt=${encodeURIComponent(first.startedAt)}`,
  );
  await admin
    .getByText("Recording triggers", { exact: true })
    .first()
    .waitFor();
  await admin.locator(".replayer-wrapper iframe").waitFor();
  await admin.waitForTimeout(2500);
  assert(
    await admin
      .locator(".replayer-wrapper iframe")
      .contentFrame()
      .getByText("User directory", { exact: true })
      .count(),
  );
  console.log("PASS rrweb playback renders the captured page");
  await shot("04-session-replay-reasons.png");
  await admin.getByRole("link", { name: /Profile inspection/ }).click();
  await admin.getByLabel("Recording trigger").waitFor();
  assert.equal(
    new URL(admin.url()).searchParams.get("triggerId"),
    triggers[1].id,
  );
  console.log("PASS session tags navigate to a shareable trigger filter");
  await api(`/sdk-config/rules/${rule.id}?${scope}`, undefined, "DELETE");
  assert.equal((await search(triggers[1].id)).pagination.total, 1);
  console.log("PASS historical recording reasons survive rule deletion");
  assert.deepEqual(errors, []);
  console.log("PASS no dashboard JavaScript errors");
  console.log("Trigger sequence E2E passed.");
} finally {
  await b.close();
}
