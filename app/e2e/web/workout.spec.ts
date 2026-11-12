import { expect, test, type Page } from "@playwright/test";

import { signInToDemo } from "./demo";

// Today's record in the demo fixtures; the browser's local date, as the app computes it.
const record = `workout-${new Date().toLocaleDateString("en-CA")}`;

// The pane shows one exercise at a time; its carousel tile brings it up. Warmups are rows like any
// other set.
const tiles = (page: Page) => page.getByTestId(/^tile-\d+$/);

async function completeShownSets(page: Page) {
  const pending = page.getByRole("checkbox", { name: /^Complete / });
  while ((await pending.count()) > 0) {
    await pending.first().click();
  }
  await expect(pending).toHaveCount(0);
  // A timed set starts its work timer on tap; Stop completes it and starts its rest.
  const timed = page.getByRole("checkbox", { name: /^Start / });
  while ((await timed.count()) > 0) {
    await timed.first().click();
    await page.getByTestId("set-timer-action").click();
  }
}

export async function completeAllSets(page: Page) {
  for (let i = 0; i < (await tiles(page).count()); i++) {
    await tiles(page).nth(i).click();
    await completeShownSets(page);
  }
}

test("logs today's workout and finishes it", async ({ page }) => {
  await signInToDemo(page);
  await page.goto(`/workout/${record}`);
  await expect(page.getByTestId("workout-screen")).toBeVisible();
  // The store is empty after a reload until the first pull lands the record.
  await expect(page.getByRole("checkbox").first()).toBeVisible();

  // The first exercise is up; the planned rest shows on its rows before anything is pressed.
  await expect(page.getByTestId("set-0-0-caption")).toHaveText(/rest \d+:\d\d/);
  await completeAllSets(page);

  // One of the tapped sets had a planned rest; the bar shows it with -10 s / +10 s and Skip.
  await expect(page.getByTestId("rest-timer")).toBeVisible();
  await page.getByTestId("rest-timer-plus").click();
  await page.getByTestId("rest-timer-minus").click();
  await page.getByTestId("rest-timer-action").click();
  await expect(page.getByTestId("rest-timer")).toHaveCount(0);

  // Inline edit on the first exercise: tapping the reps bubble opens the editor; the value lands on
  // the done set.
  await tiles(page).first().click();
  await expect(page.getByTestId("tile-0")).toHaveAccessibleName(/, (\d+) of \1 sets$/);
  const first = page.getByRole("checkbox", { name: /^Set 1 done/ }).first();
  await page
    .getByRole("button", { name: /^Reps / })
    .first()
    .click();
  await page.getByLabel("Reps", { exact: true }).fill("4");
  await expect(first).toHaveAccessibleName(/^Set 1 done, 4 reps/);

  // The Finish tile ends the carousel.
  await page.getByTestId("finish-workout").click();
  await expect(page.getByTestId("today-screen")).toBeVisible();
});

test("swipes and steps between exercises and reorders them from the Reorder sheet", async ({
  page,
}) => {
  await signInToDemo(page);
  await page.goto(`/workout/${record}`);
  await expect(page.getByRole("checkbox").first()).toBeVisible();
  const first = await tiles(page).nth(0).getAttribute("aria-label");
  const second = await tiles(page).nth(1).getAttribute("aria-label");

  await page.getByTestId("next-exercise").click();
  await expect(page.getByTestId("exercise-1")).toBeVisible();
  await tiles(page).nth(0).click();
  await expect(page.getByTestId("exercise-0")).toBeVisible();

  await page.getByTestId("reorder-button").click();
  await expect(page.getByTestId("exercises-sheet")).toBeVisible();
  const row = page.getByTestId("exercises-1");
  // The sheet slides up; a row measured on the way is below where it stops.
  await expect(row).toBeInViewport({ ratio: 1 });
  let y = -1;
  await expect
    .poll(async () => {
      const [before, now] = [y, (await row.boundingBox())!.y];
      y = now;
      return now === before;
    })
    .toBe(true);
  const box = (await row.boundingBox())!;
  const x = box.x + box.width / 3;
  await page.mouse.move(x, box.y + box.height / 2);
  await page.mouse.down();
  // Past the long press that starts a drag.
  await page.waitForTimeout(600);
  await page.mouse.move(x, box.y - box.height, { steps: 20 });
  await page.mouse.up();
  // A tile's name is the exercise and its set count, which moves with it.
  await expect(tiles(page).nth(0)).toHaveAccessibleName(second!);
  await expect(tiles(page).nth(1)).toHaveAccessibleName(first!);
});

// The web store is in memory, so a reload only keeps what the server has. Persistence across a
// reload therefore needs a real server; against mock://demo the record resets with the page.
test("a finished workout stays done while the app is open", async ({ page }) => {
  await signInToDemo(page);
  await page.goto(`/workout/${record}`);
  await expect(page.getByRole("checkbox").first()).toBeVisible();
  await completeAllSets(page);
  await page.getByTestId("finish-workout").click();
  await expect(page.getByTestId("today-screen")).toBeVisible();

  // Back into the record without a reload: Today's workout card opens the existing record.
  await page.getByRole("button", { name: "View workout" }).click();
  await expect(page.getByTestId("workout-screen")).toBeVisible();
  for (let i = 0; i < (await tiles(page).count()); i++) {
    await expect(tiles(page).nth(i)).toHaveAccessibleName(/, (\d+) of \1 sets$/);
  }
  await expect(page.getByTestId("finish-workout")).toHaveCount(0);
  await expect(page.getByText("Finished. Use Reset day to redo it.")).toBeVisible();
});
