import { execFileSync } from "node:child_process";
import { readFileSync } from "node:fs";
import path from "node:path";

import { expect, test, type Page } from "@playwright/test";

import { strength16Script } from "../../src/api/fixtures/strength16Script";

// The whole app against the compose stack: E2E_SERVER_URL=http://<server>:8080 npx playwright
// test. One long test per viewport, since the web store lives in memory and each step builds on the
// one before. Each run registers a new user with an invite minted in the server container.
const server = process.env.E2E_SERVER_URL;
const container = process.env.E2E_SERVER_CONTAINER ?? "plusultra-server-1";
const invite = () =>
  execFileSync("docker", ["exec", container, "/plusultra", "invite"]).toString().trim();
const artifacts = path.join(__dirname, "../artifacts");
const mealScript = readFileSync(
  path.join(__dirname, "../../../internal/engine/testdata/examples/meals/recomp.star"),
  "utf8",
);
// Timed, count-up, plain and unilateral sets every day, so the timer test runs on any weekday.
const timerScript = readFileSync(path.join(__dirname, "../timers.star"), "utf8");
const password = "correct horse battery";

// Local dates as the app computes them: this week's Monday and today's weekday number.
const now = new Date();
const today = now.toLocaleDateString("en-CA");
const weekday = ((now.getDay() + 6) % 7) + 1;
const mondayDate = new Date(now);
mondayDate.setDate(now.getDate() - weekday + 1);
const monday = mondayDate.toLocaleDateString("en-CA");
const tuesdayDate = new Date(mondayDate);
tuesdayDate.setDate(mondayDate.getDate() + 1);
const tuesday = tuesdayDate.toLocaleDateString("en-CA");

const dayLabel = (date: string) =>
  new Intl.DateTimeFormat("en-GB", { weekday: "long", day: "numeric", month: "long" }).format(
    new Date(`${date}T12:00:00`),
  );

test.skip(!server, "set E2E_SERVER_URL to run against a real server");
test.setTimeout(10 * 60_000);

// Connect's JSON protocol, for checking what the server stored.
async function rpc<T>(page: Page, method: string, body: unknown, token = ""): Promise<T> {
  const res = await page.request.post(`${server}/api/v1/plusultra.v1.${method}`, {
    data: body,
    headers: { "content-type": "application/json", authorization: token && `Bearer ${token}` },
  });
  expect(res.ok(), `${method}: ${await res.text()}`).toBeTruthy();
  return (await res.json()) as T;
}

// Screens below the top of a tab's stack stay mounted, each with its own Back; the top one is last.
const back = (page: Page) => page.getByRole("button", { name: "Back", exact: true }).last();

// Bottom tabs render as tabs on the web and as buttons on native.
const tab = (page: Page, name: string) =>
  page.getByRole("tab", { name, exact: true }).or(page.getByRole("button", { name, exact: true }));

async function createProgram(page: Page, script: string, expectStructure: RegExp) {
  await tab(page, "Programs").click();
  await expect(page.getByTestId("programs-screen")).toBeVisible();
  await page.getByRole("button", { name: "New" }).click();
  await expect(page.getByTestId("program-editor")).toBeVisible();
  await page.getByLabel("Script").fill(script);
  await expect(page.getByText(expectStructure)).toBeVisible({ timeout: 30_000 });
  await page.getByRole("button", { name: "Save" }).click();
  await expect(page.getByTestId("program-screen")).toBeVisible();
  await page.getByRole("button", { name: "Activate" }).click();
  const start = page.getByLabel("Start date");
  await start.fill(monday);
  await expect(page.getByText(`Starts on a Monday, at week 1 day 1`)).toBeVisible();
  await page.getByRole("button", { name: "Start program" }).click();
  await expect(page.getByText(/, Active$/)).toBeVisible({ timeout: 30_000 });
  // Pressing the current tab keeps its stack, so Back returns to the list.
  await back(page).click();
  await expect(page.getByTestId("programs-screen")).toBeVisible();
}

// Connects to the server and registers a new user with an invite; `shot` captures the filled
// server address and the empty Today.
async function signUp(
  page: Page,
  baseURL: string,
  user: { displayName: string; username: string },
  shot?: (name: string) => Promise<unknown>,
) {
  await page.goto("/");
  await expect(page.getByTestId("server-screen")).toBeVisible();
  // The page's own origin is offered first; this static server has no API behind it.
  await expect(page.getByLabel("Server address")).toHaveValue(baseURL);
  await page.getByLabel("Server address").fill(server!);
  await shot?.("01-server");
  await page.getByRole("button", { name: "Connect" }).click();
  await expect(page.getByTestId("login-screen")).toBeVisible();

  await page.getByRole("button", { name: "Create an account" }).click();
  // Sign in stays mounted under it, with the same field labels.
  const form = page.getByTestId("register-screen");
  await expect(form).toBeVisible();
  await form.getByLabel("Display name").fill(user.displayName);
  await form.getByLabel("Username").fill(user.username);
  await form.getByLabel("Password").fill(password);
  await form.getByLabel("Invite code").fill(invite());
  await form.getByRole("button", { name: "Create account" }).click();
  await expect(page.getByTestId("today-screen")).toBeVisible({ timeout: 30_000 });
  await shot?.("02-today-empty");
}

// One exercise shows at a time; its carousel tile brings it up. Warmups are rows like any other.
async function logWorkout(page: Page) {
  const tiles = page.getByTestId(/^tile-\d+$/);
  const pending = page.getByRole("checkbox", { name: /^Complete / });
  for (let i = 0; i < (await tiles.count()); i++) {
    await tiles.nth(i).click();
    while ((await pending.count()) > 0) {
      await pending.first().click();
      const skip = page.getByTestId("rest-timer-action");
      if (await skip.isVisible().catch(() => false)) await skip.click();
    }
    await expect(pending).toHaveCount(0);
  }
}

test("register, program, workout, meals, check-in and keys against the server", async ({
  page,
  baseURL,
}, { project }) => {
  const shot = (name: string) =>
    page.screenshot({
      path: path.join(artifacts, `web-${project.name}-${name}.png`),
      fullPage: true,
    });
  const user = {
    displayName: "E2E Web",
    username: `e2e_${project.name}_${Date.now().toString(36)}`,
  };

  await test.step("connect to the server and register a new user with an invite", async () => {
    await signUp(page, baseURL!, user, shot);
  });

  await test.step("create and activate the programs through the editor", async () => {
    await createProgram(page, strength16Script, /No errors\. 8 weeks, \d+ days\./);
    await shot("03-program-active");
    await createProgram(page, mealScript, /No errors\. 8 weeks, \d+ days\./);
  });

  await test.step("today shows week 1 and the planned meals", async () => {
    await tab(page, "Today").click();
    await expect(page.getByText(`Week 1, day ${weekday}`)).toBeVisible({ timeout: 30_000 });
    await expect(page.getByRole("button", { name: "Log as planned" })).toBeVisible();
    await shot("04-today-planned");
  });

  await test.step("log today's workout and let the server finalize it", async () => {
    await page.getByRole("button", { name: "Start workout" }).click();
    await expect(page.getByTestId("workout-screen")).toBeVisible();
    await logWorkout(page);
    await shot("05-workout-done");
    await page.getByTestId("finish-workout").click();
    await expect(page.getByTestId("today-screen")).toBeVisible();
    await expect(page.getByRole("button", { name: "View workout" })).toBeVisible();

    const { token } = await rpc<{ token: string }>(page, "AuthService/Login", {
      username: user.username,
      password,
    });
    await expect
      .poll(
        async () => {
          const { records } = await rpc<{ records?: { finalized: boolean }[] }>(
            page,
            "WorkoutService/ListWorkoutRecords",
            { dates: { from: today, to: today } },
            token,
          );
          // Connect JSON leaves out an empty list.
          return (records ?? []).map((r) => r.finalized);
        },
        { timeout: 30_000 },
      )
      .toEqual([true]);
  });

  await test.step("log meals as planned and a weight check-in", async () => {
    await page.getByRole("button", { name: "Log as planned" }).click();
    await expect(page.getByRole("button", { name: "Log as planned" })).toHaveCount(0);
    await page.getByRole("button", { name: "Open meals" }).click();
    await expect(page.getByTestId("meals-screen")).toBeVisible();
    await expect(page.getByText("Day total")).toBeVisible();
    await shot("06-meals");
    await back(page).click();
    await expect(page.getByTestId("today-screen")).toBeVisible();
    await page.getByLabel("Weight (kg)").fill("80.5");
    await page.getByRole("button", { name: "Save weight" }).click();
    await expect(page.getByText("Weight today")).toBeVisible();
  });

  await test.step("the calendar shows the day done and marks Tuesday skipped", async () => {
    await tab(page, "Calendar").click();
    await expect(page.getByTestId("calendar-screen")).toBeVisible();
    // The finalized record comes back once the server's pull lag has passed.
    await expect(
      page.getByRole("button", { name: `${dayLabel(today)}, done, check-in, today` }),
    ).toBeVisible({ timeout: 30_000 });
    if (weekday > 2) {
      await page.getByRole("button", { name: new RegExp(`^${dayLabel(tuesday)}`) }).click();
      await expect(page.getByTestId("day-screen")).toBeVisible();
      await page.getByRole("checkbox", { name: "Skipped" }).click();
      await back(page).click();
      await expect(
        page.getByRole("button", { name: `${dayLabel(tuesday)}, skipped` }),
      ).toBeVisible();
    }
    await shot("07-calendar");
  });

  await test.step("create and revoke an API key", async () => {
    await tab(page, "Settings").click();
    await page.getByRole("button", { name: "API keys" }).click();
    await expect(page.getByTestId("api-keys-screen")).toBeVisible();
    await page.getByRole("button", { name: "New key" }).click();
    await page.getByLabel("Name").fill("e2e");
    await page.getByRole("button", { name: "Create key" }).click();
    // The full key, shown once; the list row behind the sheet shows only its prefix.
    await expect(page.getByText(/^lsk_\w+_\w+$/)).toBeVisible();
    await shot("08-api-key");
    await page.getByRole("button", { name: "Done" }).click();
    await page.getByRole("button", { name: /^e2e/ }).click();
    await page.getByRole("button", { name: "Revoke e2e" }).click();
    await expect(page.getByText(/revoked/)).toBeVisible();
  });

  await test.step("a reload keeps the session and pulls everything back from the server", async () => {
    await page.reload();
    await expect(page.getByTestId("api-keys-screen")).toBeVisible({ timeout: 30_000 });
    await tab(page, "Today").click();
    await expect(page.getByTestId("today-screen")).toBeVisible();
    await expect(page.getByRole("button", { name: "View workout" })).toBeVisible({
      timeout: 30_000,
    });
    await expect(page.getByText("Weight today")).toBeVisible();
    await expect(page.getByText("Sync failed")).toHaveCount(0);
    await shot("09-today-reloaded");
  });
});

test("timed and unilateral sets against the server", async ({ page, baseURL }, { project }) => {
  const shot = (name: string) =>
    page.screenshot({ path: path.join(artifacts, `web-${project.name}-timers-${name}.png`) });
  const user = {
    displayName: "E2E Timers",
    username: `e2e_t_${project.name}_${Date.now().toString(36)}`,
  };
  // Sets by position in the timer program: Bench Press, Jump Rope, Plank, Bulgarian Split Squat.
  const set = (exercise: number, n: number) => page.getByTestId(`set-${exercise}-${n}-done`);
  const show = (exercise: number) => page.getByTestId(`tile-${exercise}`).click();
  const skipRest = async () => {
    await page.getByTestId("rest-timer-action").click();
    await expect(page.getByTestId("rest-timer")).toHaveCount(0);
  };

  await test.step("register and start today's workout of the timer program", async () => {
    await signUp(page, baseURL!, user);
    await createProgram(page, timerScript, /No errors\. 2 weeks, 14 days\./);
    await tab(page, "Today").click();
    await expect(page.getByText("Jump Rope")).toBeVisible({ timeout: 30_000 });
    await expect(page.getByText("3x0:45 · rest 0:30")).toBeVisible();
    await page.getByRole("button", { name: "Start workout" }).click();
    await expect(page.getByTestId("workout-screen")).toBeVisible();
  });

  await test.step("play completes a plain set and starts its rest", async () => {
    await expect(page.getByTestId("set-0-0-caption")).toHaveText("Work · rest 0:30");
    await set(0, 0).click();
    await expect(set(0, 0)).toHaveAccessibleName(/^Set 1 done/);
    await expect(page.getByTestId("rest-timer")).toContainText("Skip");
    await skipRest();
  });

  await test.step("Enter and Space start a timed set and stop it", async () => {
    await show(1);
    await set(1, 0).focus();
    await page.keyboard.press("Enter");
    const bar = page.getByTestId("set-timer");
    await expect(bar).toContainText("Jump Rope");
    await expect(bar).toContainText("Set 1");
    await expect(set(1, 0)).toHaveAccessibleName("Stop set 1");
    await shot("01-set-timer");
    await page.getByTestId("set-timer-action").focus();
    await page.keyboard.press("Enter");
    await expect(set(1, 0)).toHaveAccessibleName(/^Set 1 done/);
    await skipRest();
    await set(1, 1).focus();
    await page.keyboard.press("Space");
    await expect(set(1, 1)).toHaveAccessibleName("Stop set 2");
    await page.keyboard.press("Space");
    await expect(set(1, 1)).toHaveAccessibleName(/^Set 2 done/);
    await skipRest();
  });

  await test.step("a count-up set shows elapsed over target until stopped", async () => {
    await show(2);
    await set(2, 0).click();
    await expect(page.getByTestId("set-timer")).toContainText(/0:0\d \/ 0:30/);
    await page.getByTestId("set-timer-action").click();
    await expect(set(2, 0)).toHaveAccessibleName(/^Set 1 done/);
    await skipRest();
  });

  await test.step("a unilateral exercise logs both sides", async () => {
    await show(3);
    await page.getByRole("button", { name: "Bulgarian Split Squat options" }).click();
    await page.getByTestId("unilateral-toggle").click();
    await expect(page.getByTestId("unilateral-toggle")).toContainText("On");
    await page.keyboard.press("Escape");
    await expect(page.getByTestId("set-3-0-repsLeft")).toHaveText("L 8");
    await shot("02-unilateral");
    await set(3, 0).click();
    await expect(set(3, 0)).toHaveAccessibleName("Set 1 done, L 8 R 8, tap to undo");
    await skipRest();
  });

  await test.step("the server keeps the work times and both sides", async () => {
    await page.getByTestId("finish-workout").click();
    await expect(page.getByTestId("today-screen")).toBeVisible();
    await expect(page.getByText("per side")).toBeVisible();
    const { token } = await rpc<{ token: string }>(page, "AuthService/Login", {
      username: user.username,
      password,
    });
    type Entry = { name: string; sets: { workTime?: number; repsLeft?: number }[] };
    let entries: Entry[] = [];
    await expect
      .poll(
        async () => {
          const { records } = await rpc<{ records?: { finalized: boolean; entries: Entry[] }[] }>(
            page,
            "WorkoutService/ListWorkoutRecords",
            { dates: { from: today, to: today } },
            token,
          );
          entries = records?.[0]?.entries ?? [];
          return records?.[0]?.finalized ?? false;
        },
        { timeout: 30_000 },
      )
      .toBe(true);
    const named = (name: string) => entries.find((e) => e.name === name)!.sets[0];
    expect(named("Jump Rope").workTime).toBeGreaterThan(0);
    expect(named("Plank").workTime).toBeGreaterThan(0);
    expect(named("Bulgarian Split Squat").repsLeft).toBe(8);
  });

  await test.step("the finished workout opens editable, without a Finish button", async () => {
    await page.getByRole("button", { name: "Edit workout" }).click();
    await expect(
      page.getByText(/^Finished .*Edits update the program's next sessions\.$/),
    ).toBeVisible();
    await expect(page.getByTestId("finish-workout")).toHaveCount(0);
    await expect(page.getByRole("button", { name: "Add set" }).first()).toBeVisible();
    await shot("03-finished");
  });
});
