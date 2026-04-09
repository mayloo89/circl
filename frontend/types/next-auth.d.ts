import "next-auth"
import "next-auth/jwt"

declare module "next-auth" {
  interface Session {
    accessToken?: string
    isAdmin?: boolean
    reactivated?: boolean
  }
  interface User {
    accessToken?: string
    isAdmin?: boolean
    reactivated?: boolean
  }
}

declare module "next-auth/jwt" {
  interface JWT {
    accessToken?: string
    isAdmin?: boolean
    reactivated?: boolean
  }
}
