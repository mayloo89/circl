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
    await page.locator("#accept_terms").check()
    await page.getByRole("button", { name: "Create account" }).click()

    await expect(page.getByText("Check your email")).toBeVisible()
  })

  test("blocks registration when the terms checkbox is unchecked", async ({ page }) => {
    const ts = Date.now()
    const email = `e2e.noterms.${ts}@example.com`
    const password = "Password1!"

    await page.goto("/en/register")
    await page.locator("#email").fill(email)
    await page.locator("#username").fill(`noterms${ts}`.slice(0, 30))
    await page.getByRole("combobox", { name: "DD" }).selectOption("15")
    await page.getByRole("combobox", { name: "MM" }).selectOption("6")
    await page.getByRole("combobox", { name: "YYYY" }).selectOption("1990")
    await page.locator("#password").fill(password)
    await page.locator("#confirm").fill(password)
    // accept_terms intentionally left unchecked.
    await page.getByRole("button", { name: "Create account" }).click()

    await expect(
      page.getByText("You must accept the Terms and Privacy Policy to create an account."),
    ).toBeVisible()
    await expect(page.getByText("Check your email")).not.toBeVisible()
  })

  test("logs in with valid credentials", async ({ page, request }) => {
    const user = await createUser(request)

    await page.goto("/en/login")
    await page.locator("#email").fill(user.email)
    await page.locator("#password").fill(user.password)
    await page.getByRole("button", { name: "Sign in" }).click()

    await page.waitForURL(/\/en\/?$/)
    await expect(page.getByRole("heading", { level: 1, name: "circl" })).toBeVisible()
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
    await page.locator("#accept_terms").check()
    await page.getByRole("button", { name: "Create account" }).click()

    await expect(page.getByText("An account with this email already exists")).toBeVisible()
  })
})

// Regression: the middleware computed isLoggedIn as !!req.auth, but NextAuth
// sets req.auth to a truthy error object (not null) when session resolution
// fails, which silently opened the auth wall — anonymous visitors reached
// private routes and were redirected *away* from /login. These tests pin the
// wall's behavior in both directions.
test.describe("auth wall", () => {
  test.describe.configure({ mode: "parallel" })

  for (const path of ["/en/contacts", "/en/chat", "/en/browse", "/en/settings"]) {
    test(`redirects the anonymous visitor from ${path} to login`, async ({ page }) => {
      await page.goto(path)
      await page.waitForURL(/\/en\/login$/)
      await expect(page.locator("#email")).toBeVisible()
    })
  }

  test("keeps the login page reachable for the anonymous visitor", async ({ page }) => {
    await page.goto("/en/login")
    await expect(page).toHaveURL(/\/en\/login$/)
    await expect(page.locator("#email")).toBeVisible()
  })
})

test.describe("legal pages and footer", () => {
  test.describe.configure({ mode: "parallel" })

  for (const locale of ["en", "es", "pt"] as const) {
    test(`renders all four legal pages in ${locale}`, async ({ page }) => {
      const headings: Record<typeof locale, Record<string, string>> = {
        en: {
          terms: "Terms of Service",
          privacy: "Privacy Policy",
          guidelines: "Community Guidelines",
          safety: "Safety",
        },
        es: {
          terms: "Términos del Servicio",
          privacy: "Política de Privacidad",
          guidelines: "Pautas de la Comunidad",
          safety: "Seguridad",
        },
        pt: {
          terms: "Termos de Serviço",
          privacy: "Política de Privacidade",
          guidelines: "Diretrizes da Comunidade",
          safety: "Segurança",
        },
      }
      for (const slug of ["terms", "privacy", "guidelines", "safety"] as const) {
        await page.goto(`/${locale}/${slug}`)
        await expect(page.getByRole("heading", { level: 1, name: headings[locale][slug] })).toBeVisible()
      }
    })
  }

  test("footer on the login page links to all four legal pages", async ({ page }) => {
    await page.goto("/en/login")
    const footer = page.getByRole("navigation", { name: "Legal links" })
    await expect(footer.getByRole("link", { name: "Terms" })).toHaveAttribute("href", "/en/terms")
    await expect(footer.getByRole("link", { name: "Privacy" })).toHaveAttribute("href", "/en/privacy")
    await expect(footer.getByRole("link", { name: "Community" })).toHaveAttribute("href", "/en/guidelines")
    await expect(footer.getByRole("link", { name: "Safety" })).toHaveAttribute("href", "/en/safety")
  })

  // Regression: middleware used to redirect any non-AUTH_PAGES path to /login
  // when unauthenticated, which made the footer links from /login feel inert
  // (they bounced back to /login and the URL didn't change). Clicking through
  // here exercises the navigation end-to-end, including the auth bypass for
  // PUBLIC_PAGES.
  test("footer link from login navigates the unauthenticated visitor to /terms", async ({ page }) => {
    await page.goto("/en/login")
    await page.getByRole("navigation", { name: "Legal links" }).getByRole("link", { name: "Terms" }).click()
    await page.waitForURL(/\/en\/terms$/)
    await expect(page.getByRole("heading", { level: 1, name: "Terms of Service" })).toBeVisible()
  })
})
