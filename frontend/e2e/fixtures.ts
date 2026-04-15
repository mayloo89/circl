import { test as base, type APIRequestContext, type Page } from "@playwright/test"

const API = "http://localhost:8080"

export type TestUser = {
  id: string
  email: string
  password: string
  token: string
}

export async function createUser(request: APIRequestContext): Promise<TestUser> {
  const ts = `${Date.now()}.${Math.random().toString(36).slice(2, 6)}`
  const email = `e2e.${ts}@example.com`
  const password = "Password1!"
  const username = `e2e_${ts.replace(".", "_")}`.slice(0, 30)

  const res = await request.post(`${API}/test/users`, {
    data: { email, password, username },
  })
  if (!res.ok()) {
    throw new Error(`Failed to create test user: ${await res.text()}`)
  }
  return res.json()
}

export async function loginAs(page: Page, user: TestUser): Promise<void> {
  await page.goto("/en/login")
  await page.locator("#email").fill(user.email)
  await page.locator("#password").fill(user.password)
  await page.getByRole("button", { name: "Sign in" }).click()
  await page.waitForURL(/\/en\/?$/)
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
