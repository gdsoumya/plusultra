import { expect, type Page } from "@playwright/test";

// The demo server through the auth screens; a signed-in page skips this. The exported web build
// has no "Try the demo" button (dev only), so the address is typed instead.
export async function signInToDemo(page: Page) {
  await page.goto("/");
  const address = page.getByLabel("Server address");
  const today = page.getByTestId("today-screen");
  await expect(address.or(today)).toBeVisible();
  if (await address.isVisible()) {
    await address.fill("mock://demo");
    await page.getByRole("button", { name: "Connect" }).click();
    await page.getByLabel("Username").fill("demo");
    await page.getByLabel("Password").fill("demo-password");
    await page.getByRole("button", { name: "Sign in" }).click();
  }
  await expect(today).toBeVisible();
}
