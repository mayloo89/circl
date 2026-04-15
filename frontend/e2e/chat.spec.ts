import { test, expect, createUser, loginAs } from "./fixtures"

test.describe("chat", () => {
  test("sends a message and both users see it in real time", async ({ browser, request }) => {
    const userA = await createUser(request)
    const userB = await createUser(request)

    // Establish contact: A sends, B accepts
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

    // Both users open the DM room
    await pageA.reload()
    await pageA.getByRole("button", { name: "Message" }).first().click()
    await pageA.waitForURL(/\/chat\//)

    await pageB.getByRole("button", { name: "Message" }).first().click()
    await pageB.waitForURL(/\/chat\//)

    // Wait for the WebSocket connection to be established on both sides
    await pageA.waitForSelector("#message-input:not([disabled])", { timeout: 10_000 })

    // User A sends a message
    const message = `hello-${Date.now()}`
    await pageA.locator("#message-input").fill(message)
    await pageA.getByRole("button", { name: "Send" }).click()

    // Verify the message appears for both users
    await expect(pageA.getByText(message)).toBeVisible()
    await expect(pageB.getByText(message)).toBeVisible({ timeout: 10_000 })

    await ctxA.close()
    await ctxB.close()
  })
})
