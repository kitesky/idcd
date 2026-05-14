import { redirect } from "next/navigation"

// Redirect to main alerts page — policies are in the Policies tab
export default function PoliciesPage() {
  redirect("/app/alerts")
}
