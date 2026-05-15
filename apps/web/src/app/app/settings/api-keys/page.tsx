import { APIKeysClient } from "./api-keys-client"

export const metadata = {
  title: "API Keys — idcd",
}

export default function APIKeysPage() {
  return (
    <main className="flex-1 container max-w-4xl">
      <APIKeysClient />
    </main>
  )
}
