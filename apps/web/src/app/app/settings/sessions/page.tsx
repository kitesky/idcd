import { SessionsClient } from "./sessions-client"

export const metadata = {
  title: "活跃会话 — idcd",
}

export default function SessionsPage() {
  return (
    <main className="flex-1 container max-w-4xl">
      <SessionsClient />
    </main>
  )
}
