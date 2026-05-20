import type { Metadata } from "next"
import { CertDetailClient } from "./cert-detail-client"

type Props = { params: Promise<{ id: string }> }

export async function generateMetadata({ params }: Props): Promise<Metadata> {
  const { id } = await params
  return {
    title: `证书 ${id} - idcd`,
    description: "TLS 证书详情",
  }
}

export default async function CertDetailPage({ params }: Props) {
  const { id } = await params
  return <CertDetailClient certId={id} />
}
