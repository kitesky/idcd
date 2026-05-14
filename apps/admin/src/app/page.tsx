import { redirect } from "next/navigation"

// Redirect root to nodes dashboard
export default function Home() {
  redirect("/nodes")
}
