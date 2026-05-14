"use client"

import { ThemeProvider as NextThemesProvider } from "next-themes"
import type { ThemeProviderProps } from "next-themes"

interface CustomThemeProviderProps extends ThemeProviderProps {
  children: React.ReactNode
  nonce?: string
}

export function ThemeProvider({ children, nonce, ...props }: CustomThemeProviderProps) {
  return (
    <NextThemesProvider
      attribute="class"
      defaultTheme="dark"
      enableSystem={false}
      nonce={nonce}
      {...props}
    >
      {children}
    </NextThemesProvider>
  )
}