import { test, expect } from "@playwright/test"
import { createUser } from "./fixtures"

test.describe("auth", () => {
  test("registers a new account and shows email verification screen", async ({ page }) => {
    const ts = Date.now()
    const email = `e2e.new.${ts}@example.com`
    const password = "Password1!"

    await page.goto("/en/register")
    await page.locator("#email").fill(email)
    await page.locator("#username").fill(`newuser${ts}`.slice(0, 30))
    await page.getByRole("combobox", { name: "DD" }).selectOption("15")
    await page.getByRole("combobox", { name: "MM" }).selectOption("6")
    await page.getByRole("combobox", { name: "YYYY" }).selectOption("1990")
    await page.locator("#password").fill(password)
    await page.locator("#confirm").fill(password)
    await page.getByRole("button", { name: "Create account" }).click()

    await expect(page.getByText("Check your email")).toBeVisible()
  })

  test("logs in with valid credentials", async ({ page, request }) => {
    const user = await createUser(request)

    await page.goto("/en/login")
    await page.locator("#email").fill(user.email)
    await page.locator("#password").fill(user.password)
    await page.getByRole("button", { name: "Sign in" }).click()

    await page.waitForURL(/\/en\/?$/)
    await expect(page.getByText("Welcome to Circl")).toBeVisible()
  })

  test("shows error for wrong password", async ({ page, request }) => {
    const user = await createUser(request)

    await page.goto("/en/login")
    await page.locator("#email").fill(user.email)
    await page.locator("#password").fill("WrongPassword99!")
    await page.getByRole("button", { name: "Sign in" }).click()

    await expect(page.getByText("Invalid email or password.")).toBeVisible()
  })

  test("shows error when registering a duplicate email", async ({ page, request }) => {
    const user = await createUser(request)
    const ts = Date.now()

    await page.goto("/en/register")
    await page.locator("#email").fill(user.email)
    await page.locator("#username").fill(`dupuser${ts}`.slice(0, 30))
    await page.getByRole("combobox", { name: "DD" }).selectOption("15")
    await page.getByRole("combobox", { name: "MM" }).selectOption("6")
    await page.getByRole("combobox", { name: "YYYY" }).selectOption("1990")
    await page.locator("#password").fill(user.password)
    await page.locator("#confirm").fill(user.password)
    await page.getByRole("button", { name: "Create account" }).click()

    await expect(page.getByText("An account with this email already exists")).toBeVisible()
  })
})
