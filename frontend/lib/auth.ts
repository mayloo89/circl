import NextAuth from "next-auth"
import CredentialsProvider from "next-auth/providers/credentials"

const BACKEND_URL = process.env.BACKEND_URL ?? "http://localhost:8080"

function jwtExp(accessToken: string): number | null {
  try {
    const [, payload] = accessToken.split(".")
    const { exp } = JSON.parse(atob(payload.replace(/-/g, "+").replace(/_/g, "/")))
    return typeof exp === "number" ? exp : null
  } catch {
    return null
  }
}

export const { handlers, signIn, signOut, auth } = NextAuth({
  providers: [
    CredentialsProvider({
      name: "Credentials",
      credentials: {
        email: { label: "Email", type: "email" },
        password: { label: "Password", type: "password" },
        rememberMe: { label: "Remember me", type: "text" },
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
              remember_me: credentials.rememberMe === "true",
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
          const exp = jwtExp(user.token)
          try {
            const payload = JSON.parse(atob(user.token.split(".")[1].replace(/-/g, "+").replace(/_/g, "/")))
            if (typeof payload.role === "string") role = payload.role
          } catch { /* ignore malformed token */ }
          return {
            id: user.id,
            email: user.email,
            name: user.email,
            accessToken: user.token,
            refreshToken: user.refresh_token ?? undefined,
            role,
            reactivated: user.reactivated ?? false,
            exp,
          }
        } catch (err) {
          if (err instanceof Error && ["AccountLocked", "RateLimited", "EmailNotVerified"].includes(err.message)) {
            throw err
          }
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
      // Fresh login — populate token from the user object returned by authorize().
      if (user) {
        token.id = user.id
        token.accessToken = user.accessToken
        token.refreshToken = user.refreshToken
        token.role = user.role
        token.reactivated = user.reactivated
        token.error = undefined
        return token
      }

      // Access token still valid — nothing to do.
      if (token.accessToken) {
        const exp = jwtExp(token.accessToken as string)
        if (exp !== null && exp * 1000 > Date.now()) {
          return token
        }
      }

      // Access token expired — attempt silent refresh.
      // On either a non-OK response or a network error we clear the stored
      // refresh token so the next JWT callback short-circuits to RefreshFailed
      // instead of replaying the same already-rejected token in a tight loop.
      if (token.refreshToken) {
        try {
          const res = await fetch(`${BACKEND_URL}/auth/refresh`, {
            method: "POST",
            headers: { "Content-Type": "application/json" },
            body: JSON.stringify({ refresh_token: token.refreshToken }),
          })
          if (!res.ok) {
            token.refreshToken = undefined
            token.error = "RefreshFailed"
            return token
          }
          const data = await res.json()
          token.accessToken = data.token
          token.refreshToken = data.refresh_token ?? token.refreshToken
          token.error = undefined
          return token
        } catch {
          token.refreshToken = undefined
          token.error = "RefreshFailed"
          return token
        }
      }

      token.error = "TokenExpired"
      return token
    },
    async session({ session, token }) {
      if (session.user) {
        session.user.id = token.id as string
      }
      session.accessToken = token.accessToken
      session.refreshToken = token.refreshToken
      session.role = token.role as string | undefined
      session.reactivated = token.reactivated as boolean | undefined
      session.error = token.error as string | undefined
      return session
    },
  },
  session: {
    strategy: "jwt",
  },
})
