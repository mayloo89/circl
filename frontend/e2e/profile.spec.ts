import { test, expect } from "./fixtures"

test.describe("profile", () => {
  test("shows My Profile page after login", async ({ authenticatedPage: { page } }) => {
    await page.goto("/profile")
    await expect(page.getByRole("heading", { name: "My Profile" })).toBeVisible()
  })

  test("updates display name and shows success message", async ({ authenticatedPage: { page } }) => {
    await page.goto("/profile")

    const newName = `E2E User ${Date.now()}`
    await page.locator("#displayName").fill(newName)
    await page.getByRole("button", { name: "Save changes" }).click()

    await expect(page.getByText("Profile updated.")).toBeVisible()
  })

  test("navigates home from profile page", async ({ authenticatedPage: { page } }) => {
    await page.goto("/profile")
    await page.getByRole("button", { name: /home/i }).click()
    await page.waitForURL("/")
    await expect(page.getByText("Welcome to Circl")).toBeVisible()
  })
})
