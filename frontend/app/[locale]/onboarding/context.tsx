"use client"

import { createContext, useContext } from "react"

interface OnboardingContextValue {
  skipStep: (nextPath: string) => Promise<void>
}

export const OnboardingContext = createContext<OnboardingContextValue>({
  skipStep: () => Promise.resolve(),
})

export function useOnboardingContext() {
  return useContext(OnboardingContext)
}
