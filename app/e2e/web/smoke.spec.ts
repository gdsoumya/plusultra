import { expect, test } from "@playwright/test";

import { signInToDemo } from "./demo";

test("today screen renders", async ({ page }) => {
  await signInToDemo(page);
  await expect(page.getByText("Today", { exact: true }).first()).toBeVisible();
});
