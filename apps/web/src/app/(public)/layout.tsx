import { Nav } from "@/components/nav"
import { Footer } from "@/components/footer"
import { BackToTop } from "@/components/back-to-top"

export default function PublicLayout({ children }: { children: React.ReactNode }) {
  return (
    <>
      <Nav />
      {children}
      <Footer />
      <BackToTop />
    </>
  )
}
