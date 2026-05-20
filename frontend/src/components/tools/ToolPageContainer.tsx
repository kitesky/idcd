import { cn } from "@/lib/utils"

interface ToolPageContainerProps {
  children: React.ReactNode
  className?: string
}

export function ToolPageContainer({ children, className }: ToolPageContainerProps) {
  return (
    <div className={cn(
      "w-full mx-auto max-w-screen-xl px-6 py-12 md:py-16",
      className
    )}>
      {children}
    </div>
  )
}
