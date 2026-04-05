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

          if (res.status === 429) throw new Error("AccountLocked")
          if (!res.ok) return null

          const user = await res.json()
          let isAdmin = false
          try {
            const payload = JSON.parse(atob(user.token.split(".")[1].replace(/-/g, "+").replace(/_/g, "/")))
            isAdmin = payload.is_admin === true
          } catch { /* ignore malformed token */ }
          return { id: user.id, email: user.email, name: user.email, accessToken: user.token, isAdmin }
        } catch {
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
        token.isAdmin = user.isAdmin
      }
      return token
    },
    async session({ session, token }) {
      if (session.user) {
        session.user.id = token.id as string
      }
      session.accessToken = token.accessToken
      session.isAdmin = token.isAdmin as boolean | undefined
      return session
    },
  },
  session: {
    strategy: "jwt",
  },
})
