import { Nav } from "@/components/nav"
import { Footer } from "@/components/footer"

export default function PublicLayout({ children }: { children: React.ReactNode }) {
  return (
    <>
      <Nav />
      {children}
      <Footer />
    </>
  )
}
