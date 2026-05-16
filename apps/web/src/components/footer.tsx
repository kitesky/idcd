"use client"

import { useState } from "react"
import Link from "next/link"
import { ChevronDown } from "lucide-react"
import { useTranslations } from "next-intl"
import { cn } from "@/lib/utils"

interface FooterLink {
  name: string
  href: string
}

function AccordionSection({ title, links }: { title: string; links: FooterLink[] }) {
  const [open, setOpen] = useState(false)
  return (
    <div className="border-b md:border-b-0">
      {/* mobile: tap to toggle */}
      <button
        className="flex w-full items-center justify-between py-4 text-sm font-semibold text-foreground md:hidden"
        onClick={() => setOpen(v => !v)}
      >
        {title}
        <ChevronDown className={cn("h-4 w-4 text-muted-foreground transition-transform duration-200", open && "rotate-180")} />
      </button>
      {/* mobile collapsed content */}
      <div className={cn("overflow-hidden transition-all duration-200 md:hidden", open ? "max-h-96 pb-4" : "max-h-0")}>
        <ul className="space-y-3">
          {links.map(link => (
            <li key={link.name}>
              <Link href={link.href as any} className="text-sm text-muted-foreground hover:text-foreground transition-colors">
                {link.name}
              </Link>
            </li>
          ))}
        </ul>
      </div>
      {/* desktop: always visible */}
      <div className="hidden md:block">
        <p className="text-sm font-semibold text-foreground mb-3">{title}</p>
        <ul className="space-y-2.5">
          {links.map(link => (
            <li key={link.name}>
              <Link href={link.href as any} className="text-sm text-muted-foreground hover:text-foreground transition-colors">
                {link.name}
              </Link>
            </li>
          ))}
        </ul>
      </div>
    </div>
  )
}

export function Footer() {
  const t = useTranslations("nav")

  const sections = [
    {
      title: t("footer.sections.about.title"),
      links: [
        { name: t("footer.sections.about.whyIdcd"), href: "/about" },
        { name: t("footer.sections.about.docs"), href: "/docs/api" },
        { name: t("footer.sections.about.agent"), href: "/agent" },
        { name: t("footer.sections.about.transparency"), href: "/transparency" },
        { name: t("footer.sections.about.join"), href: "/about" },
      ],
    },
    {
      title: t("footer.sections.probe.title"),
      links: [
        { name: t("footer.sections.probe.ping"), href: "/tools/ping" },
        { name: t("footer.sections.probe.http"), href: "/tools/http" },
        { name: t("footer.sections.probe.dns"), href: "/tools/dns" },
        { name: t("footer.sections.probe.tcp"), href: "/tools/tcp" },
        { name: t("footer.sections.probe.traceroute"), href: "/tools/traceroute" },
        { name: t("footer.sections.probe.mtr"), href: "/tools/mtr" },
      ],
    },
    {
      title: t("footer.sections.monitoring.title"),
      links: [
        { name: t("footer.sections.monitoring.monitors"), href: "/app/monitors" },
        { name: t("footer.sections.monitoring.alerts"), href: "/app/alerts" },
        { name: t("footer.sections.monitoring.reports"), href: "/app/reports" },
      ],
    },
  ]

  return (
    <footer className="border-t bg-background">
      <div className="mx-auto max-w-screen-xl px-4 sm:px-6 lg:px-8 pt-0 md:pt-10 pb-0">

        {/* Desktop: 5-col grid | Mobile: stacked accordion */}
        <div className="md:grid md:grid-cols-[180px_1fr_1fr_1fr_200px] md:gap-x-6 md:gap-y-6">

          {/* Logo — desktop only top section; on mobile shown above accordion */}
          <div className="py-8 md:py-0 md:col-span-1">
            <a href="/" className="inline-flex items-center gap-2">
              <svg className="h-6 w-6 text-primary" viewBox="0 0 24 24" fill="currentColor">
                <path d="M3 12L12 3l9 9v9H3v-9z" opacity="0.2" />
                <path d="M3 12L12 3l9 9" fill="none" stroke="currentColor" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round" />
              </svg>
              <span className="font-mono font-bold text-foreground text-lg tracking-tight">idcd</span>
            </a>
          </div>

          {/* Accordion sections */}
          {sections.map(s => (
            <AccordionSection key={s.title} title={s.title} links={s.links} />
          ))}

          {/* 联系我们 — always expanded */}
          <div className="py-6 md:py-0 border-b md:border-b-0">
            <p className="text-sm font-semibold text-foreground mb-3">{t("footer.sections.contact.title")}</p>
            <p className="text-sm text-muted-foreground mb-1">
              {t("footer.sections.contact.email")}
              <a href="mailto:hi@idcd.com" className="hover:text-foreground transition-colors">hi@idcd.com</a>
            </p>
            <div className="flex gap-4 mt-3 flex-wrap">
              {[
                { label: t("footer.sections.contact.wechat"), text: t("footer.sections.contact.wechatQr") },
                { label: t("footer.sections.contact.video"), text: t("footer.sections.contact.videoQr") },
              ].map(({ label, text }) => (
                <div key={label} className="flex flex-col items-center gap-1.5 group relative">
                  {/* 放大气泡 */}
                  <div className="absolute bottom-full left-1/2 -translate-x-1/2 mb-2 opacity-0 scale-90 group-hover:opacity-100 group-hover:scale-100 transition-all duration-200 pointer-events-none z-50">
                    <div className="h-[140px] w-[140px] rounded-lg bg-popover border border-border shadow-xl flex items-center justify-center">
                      <span className="text-[11px] text-muted-foreground text-center leading-relaxed whitespace-pre-line px-2">{text}</span>
                    </div>
                    {/* 小三角 */}
                    <div className="absolute top-full left-1/2 -translate-x-1/2 w-0 h-0 border-l-[6px] border-r-[6px] border-t-[6px] border-l-transparent border-r-transparent border-t-border" />
                  </div>
                  {/* 小缩略图 */}
                  <div className="h-[72px] w-[72px] rounded-md bg-muted/60 border border-border flex items-center justify-center shrink-0 cursor-pointer transition-colors group-hover:border-primary/50">
                    <span className="text-[9px] text-muted-foreground text-center leading-tight px-1 whitespace-pre-line">{text}</span>
                  </div>
                  <span className="text-xs text-muted-foreground">{label}</span>
                </div>
              ))}
            </div>
          </div>
        </div>

        {/* Bottom bar */}
        <div className="mt-0 md:mt-8 py-4 border-t flex flex-col sm:flex-row items-center justify-between gap-3 text-xs text-muted-foreground">
          <span>{t("footer.copyright")}</span>
          <div className="flex items-center gap-4 flex-wrap justify-center sm:justify-end">
            <a href="https://beian.miit.gov.cn/" target="_blank" rel="noopener noreferrer" className="hover:text-foreground transition-colors">
              {t("footer.icp")}
            </a>
            <a href="https://www.beian.gov.cn/portal/registerSystemInfo?recordcode=51010702001950" target="_blank" rel="noopener noreferrer" className="hover:text-foreground transition-colors">
              {t("footer.beian")}
            </a>
          </div>
        </div>

      </div>
    </footer>
  )
}
