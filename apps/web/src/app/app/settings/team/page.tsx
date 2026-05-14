import { TeamClient } from "./team-client"

export const metadata = {
  title: "团队 — idcd",
}

export default function TeamPage() {
  return (
    <main className="flex-1 container max-w-4xl py-8">
      <TeamClient />
    </main>
  )
}
