import { redirect } from "next/navigation"

// Redirect to main alerts page — channels are in the Channels tab
export default function ChannelsPage() {
  redirect("/app/alerts")
}
