import { test, expect, createUser, loginAs } from "./fixtures"

test.describe("contacts", () => {
  test("can search for another user by email", async ({ authenticatedPage: { page }, request }) => {
    const userB = await createUser(request)

    await page.goto("/en/contacts")
    await page.locator("#contact-search").fill(userB.email)

    await expect(page.getByRole("button", { name: "Add" }).first()).toBeVisible()
  })

  test("can send and accept a contact request", async ({ browser, request }) => {
    const userA = await createUser(request)
    const userB = await createUser(request)

    // User A sends a request to User B
    const ctxA = await browser.newContext()
    const pageA = await ctxA.newPage()
    await loginAs(pageA, userA)
    await pageA.goto("/en/contacts")
    await pageA.locator("#contact-search").fill(userB.email)
    await pageA.getByRole("button", { name: "Add" }).first().click()
    await expect(pageA.getByText("Sent Requests")).toBeVisible()

    // User B accepts the request
    const ctxB = await browser.newContext()
    const pageB = await ctxB.newPage()
    await loginAs(pageB, userB)
    await pageB.goto("/en/contacts")
    await expect(pageB.getByText("Pending Requests")).toBeVisible()
    await pageB.getByRole("button", { name: "Accept" }).first().click()

    // User B now sees User A in their contact list
    await expect(pageB.getByText("My Contacts (1)")).toBeVisible()

    await ctxA.close()
    await ctxB.close()
  })

  test("can open a DM from the contacts page", async ({ browser, request }) => {
    const userA = await createUser(request)
    const userB = await createUser(request)

    // Establish contact relationship
    const ctxA = await browser.newContext()
    const pageA = await ctxA.newPage()
    await loginAs(pageA, userA)
    await pageA.goto("/en/contacts")
    await pageA.locator("#contact-search").fill(userB.email)
    await pageA.getByRole("button", { name: "Add" }).first().click()

    const ctxB = await browser.newContext()
    const pageB = await ctxB.newPage()
    await loginAs(pageB, userB)
    await pageB.goto("/en/contacts")
    await pageB.getByRole("button", { name: "Accept" }).first().click()

    // User A refreshes contacts and opens the DM
    await pageA.reload()
    await pageA.getByRole("button", { name: "Message" }).first().click()
    await pageA.waitForURL(/\/chat\//)

    await ctxA.close()
    await ctxB.close()
  })
})
