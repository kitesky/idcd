"use client"

import { useEffect } from "react"
import { useRouter } from "next/navigation"
import { apiRequest } from "@/lib/api"

export default function LogoutPage() {
  const router = useRouter()

  useEffect(() => {
    async function logout() {
      try {
        await apiRequest("/v1/auth/logout", { method: "POST" })
      } catch (err) {
        console.error("Logout error:", err)
      } finally {
        // Cookie is cleared server-side by the logout endpoint.
        router.push("/")
      }
    }

    logout()
  }, [router])

  return (
    <div className="flex-1 flex items-center justify-center">
      <div className="text-center space-y-4">
        <h1 className="text-2xl font-semibold">正在登出...</h1>
        <p className="text-muted-foreground">请稍候</p>
      </div>
    </div>
  )
}
