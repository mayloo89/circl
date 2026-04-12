import NextAuth from "next-auth"
import CredentialsProvider from "next-auth/providers/credentials"

const BACKEND_URL = process.env.BACKEND_URL ?? "http://localhost:8080"

export const { handlers, signIn, signOut, auth } = NextAuth({
  providers: [
    CredentialsProvider({
      name: "Credentials",
      credentials: {
        email: { label: "Email", type: "email" },
        password: { label: "Password", type: "password" },
      },
      async authorize(credentials) {
        if (!credentials?.email || !credentials?.password) return null

        try {
          const res = await fetch(`${BACKEND_URL}/auth/login`, {
            method: "POST",
            headers: { "Content-Type": "application/json" },
            body: JSON.stringify({
              email: credentials.email,
              password: credentials.password,
            }),
          })

          if (res.status === 429) {
            const data = await res.json().catch(() => ({}))
            const msg = ((data as { error?: string }).error ?? "").toLowerCase()
            if (msg.includes("account temporarily locked")) throw new Error("AccountLocked")
            throw new Error("RateLimited")
          }
          if (res.status === 403) throw new Error("EmailNotVerified")
          if (!res.ok) return null

          const user = await res.json()
          let role = "user"
          try {
            const payload = JSON.parse(atob(user.token.split(".")[1].replace(/-/g, "+").replace(/_/g, "/")))
            if (typeof payload.role === "string") role = payload.role
          } catch { /* ignore malformed token */ }
          return { id: user.id, email: user.email, name: user.email, accessToken: user.token, role, reactivated: user.reactivated ?? false }
        } catch (err) {
          if (err instanceof Error && ["AccountLocked", "RateLimited", "EmailNotVerified"].includes(err.message)) {
            throw err
          }
          // Backend unavailable — fail closed (do not grant access)
          return null
        }
      },
    }),
  ],
  pages: {
    signIn: "/login",
  },
  callbacks: {
    async jwt({ token, user }) {
      if (user) {
        token.id = user.id
        token.accessToken = user.accessToken
        token.role = user.role
        token.reactivated = user.reactivated
      }
      return token
    },
    async session({ session, token }) {
      if (session.user) {
        session.user.id = token.id as string
      }
      session.accessToken = token.accessToken
      session.role = token.role as string | undefined
      session.reactivated = token.reactivated as boolean | undefined
      return session
    },
  },
  session: {
    strategy: "jwt",
  },
})
