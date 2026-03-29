import { test as base, type APIRequestContext, type Page } from "@playwright/test"

const API = "http://localhost:8080"

export type TestUser = {
  id: string
  email: string
  password: string
}

export async function createUser(request: APIRequestContext): Promise<TestUser> {
  const ts = `${Date.now()}.${Math.random().toString(36).slice(2, 6)}`
  const email = `e2e.${ts}@example.com`
  const password = "Password1!"

  const res = await request.post(`${API}/auth/register`, {
    data: { email, password },
  })
  if (!res.ok()) {
    throw new Error(`Failed to register test user: ${await res.text()}`)
  }
  const data = await res.json()
  return { id: data.id, email, password }
}

export async function loginAs(page: Page, user: TestUser): Promise<void> {
  await page.goto("/login")
  await page.locator("#email").fill(user.email)
  await page.locator("#password").fill(user.password)
  await page.getByRole("button", { name: "Sign in" }).click()
  await page.waitForURL("/")
}

type E2EFixtures = {
  authenticatedPage: { page: Page; user: TestUser }
}

export const test = base.extend<E2EFixtures>({
  authenticatedPage: async ({ page, request }, provide) => {
    const user = await createUser(request)
    await loginAs(page, user)
    await provide({ page, user })
  },
})

export { expect } from "@playwright/test"
