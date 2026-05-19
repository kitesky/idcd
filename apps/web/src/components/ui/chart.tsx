"use client"

import * as React from "react"
import * as RechartsPrimitive from "recharts"

import { cn } from "@/lib/utils"

// shadcn/ui chart wrappers around Recharts (see https://ui.shadcn.com/charts).
// chartConfig drives both the per-series colours (injected as CSS custom
// properties like --color-<key>) and the tooltip / legend labels — so series
// definitions and visual styling stay in one place. Theme variants supply
// distinct hex/oklch values per light/dark scheme.
//
// Typed loosely on tooltip/legend payload because recharts 3.x ships strict
// per-state shapes for these (TooltipContentProps, LegendPayload) that
// re-export awkwardly through generics — `unknown[]` + per-item guards keeps
// us out of the type-fight without losing safety where it matters.

const THEMES = { light: "", dark: ".dark" } as const

export type ChartConfig = {
  [k in string]: {
    label?: React.ReactNode
    icon?: React.ComponentType
  } & (
    | { color?: string; theme?: never }
    | { color?: never; theme: Record<keyof typeof THEMES, string> }
  )
}

type ChartContextProps = { config: ChartConfig }
const ChartContext = React.createContext<ChartContextProps | null>(null)

function useChart() {
  const ctx = React.useContext(ChartContext)
  if (!ctx) throw new Error("useChart must be used within a <ChartContainer />")
  return ctx
}

interface ChartContainerProps extends React.ComponentProps<"div"> {
  config: ChartConfig
  children: React.ReactElement
}

const ChartContainer = React.forwardRef<HTMLDivElement, ChartContainerProps>(
  ({ id, className, children, config, ...props }, ref) => {
    const uniqueId = React.useId()
    const chartId = `chart-${id ?? uniqueId.replace(/:/g, "")}`

    return (
      <ChartContext.Provider value={{ config }}>
        <div
          data-chart={chartId}
          ref={ref}
          className={cn(
            "flex aspect-video justify-center text-xs",
            "[&_.recharts-cartesian-axis-tick_text]:fill-muted-foreground",
            "[&_.recharts-cartesian-grid_line]:stroke-border/50",
            "[&_.recharts-curve.recharts-tooltip-cursor]:stroke-border",
            "[&_.recharts-dot]:stroke-transparent",
            "[&_.recharts-layer]:outline-none",
            "[&_.recharts-polar-grid_[stroke='#ccc']]:stroke-border",
            "[&_.recharts-radial-bar-background-sector]:fill-muted",
            "[&_.recharts-rectangle.recharts-tooltip-cursor]:fill-muted",
            "[&_.recharts-reference-line_[stroke='#ccc']]:stroke-border",
            "[&_.recharts-sector[stroke='#fff']]:stroke-transparent",
            "[&_.recharts-sector]:outline-none",
            "[&_.recharts-surface]:outline-none",
            className
          )}
          {...props}
        >
          <ChartStyle id={chartId} config={config} />
          <RechartsPrimitive.ResponsiveContainer>{children}</RechartsPrimitive.ResponsiveContainer>
        </div>
      </ChartContext.Provider>
    )
  }
)
ChartContainer.displayName = "Chart"

const ChartStyle = ({ id, config }: { id: string; config: ChartConfig }) => {
  const colorConfig = Object.entries(config).filter(([, c]) => c.theme || c.color)
  if (!colorConfig.length) return null
  return (
    <style
      // eslint-disable-next-line react/no-danger
      dangerouslySetInnerHTML={{
        __html: Object.entries(THEMES)
          .map(
            ([theme, prefix]) => `
${prefix} [data-chart=${id}] {
${colorConfig
  .map(([key, item]) => {
    const color =
      ("theme" in item ? item.theme?.[theme as keyof typeof THEMES] : undefined) ?? item.color
    return color ? `  --color-${key}: ${color};` : ""
  })
  .join("\n")}
}
`
          )
          .join("\n"),
      }}
    />
  )
}

const ChartTooltip = RechartsPrimitive.Tooltip

type TooltipPayloadItem = {
  dataKey?: string | number
  name?: string | number
  value?: number | string | (number | string)[]
  color?: string
  payload?: Record<string, unknown>
}

interface ChartTooltipContentProps extends React.HTMLAttributes<HTMLDivElement> {
  active?: boolean
  payload?: unknown
  label?: unknown
  hideLabel?: boolean
  hideIndicator?: boolean
  indicator?: "line" | "dot" | "dashed"
  nameKey?: string
  labelKey?: string
  labelClassName?: string
  labelFormatter?: (value: unknown, payload: TooltipPayloadItem[]) => React.ReactNode
  formatter?: (
    value: TooltipPayloadItem["value"],
    name: TooltipPayloadItem["name"],
    item: TooltipPayloadItem,
    index: number,
    payload: TooltipPayloadItem[]
  ) => React.ReactNode
  color?: string
}

const ChartTooltipContent = React.forwardRef<HTMLDivElement, ChartTooltipContentProps>(
  (
    {
      active,
      payload,
      className,
      indicator = "dot",
      hideLabel = false,
      hideIndicator = false,
      label,
      labelFormatter,
      labelClassName,
      formatter,
      color,
      nameKey,
      labelKey,
    },
    ref
  ) => {
    const { config } = useChart()
    const items = Array.isArray(payload) ? (payload as TooltipPayloadItem[]) : []

    const tooltipLabel = React.useMemo(() => {
      if (hideLabel || items.length === 0) return null
      const [item] = items
      const key = `${labelKey || item?.dataKey || item?.name || "value"}`
      const itemConfig = getPayloadConfig(config, item, key)
      const value =
        !labelKey && typeof label === "string"
          ? (config[label]?.label ?? label)
          : itemConfig?.label
      if (labelFormatter && value !== undefined) {
        return <div className={cn("font-medium", labelClassName)}>{labelFormatter(value, items)}</div>
      }
      if (value === undefined || value === null) return null
      return <div className={cn("font-medium", labelClassName)}>{value as React.ReactNode}</div>
    }, [label, labelFormatter, items, hideLabel, labelClassName, config, labelKey])

    if (!active || items.length === 0) return null
    const nestLabel = items.length === 1 && indicator !== "dot"

    return (
      <div
        ref={ref}
        className={cn(
          "grid min-w-[8rem] items-start gap-1.5 rounded-lg border border-border/60 bg-popover px-2.5 py-1.5 text-xs shadow-xl",
          className
        )}
      >
        {!nestLabel ? tooltipLabel : null}
        <div className="grid gap-1.5">
          {items.map((item, index) => {
            const key = `${nameKey || item.name || item.dataKey || "value"}`
            const itemConfig = getPayloadConfig(config, item, key)
            const indicatorColor =
              color || (item.payload as { fill?: string })?.fill || item.color

            return (
              <div
                key={`${item.dataKey ?? index}`}
                className={cn(
                  "flex w-full flex-wrap items-stretch gap-2 [&>svg]:h-2.5 [&>svg]:w-2.5 [&>svg]:text-muted-foreground",
                  indicator === "dot" && "items-center"
                )}
              >
                {formatter && item.value !== undefined && item.name ? (
                  formatter(item.value, item.name, item, index, items)
                ) : (
                  <>
                    {itemConfig?.icon ? (
                      <itemConfig.icon />
                    ) : (
                      !hideIndicator && (
                        <div
                          className={cn("shrink-0 rounded-[2px]", {
                            "h-2.5 w-2.5": indicator === "dot",
                            "w-1": indicator === "line",
                            "w-0 border-[1.5px] border-dashed bg-transparent":
                              indicator === "dashed",
                            "my-0.5": nestLabel && indicator === "dashed",
                          })}
                          style={{
                            background: indicatorColor,
                            borderColor: indicatorColor,
                          }}
                        />
                      )
                    )}
                    <div
                      className={cn(
                        "flex flex-1 justify-between leading-none",
                        nestLabel ? "items-end" : "items-center"
                      )}
                    >
                      <div className="grid gap-1.5">
                        {nestLabel ? tooltipLabel : null}
                        <span className="text-muted-foreground">{itemConfig?.label || item.name}</span>
                      </div>
                      {item.value !== undefined && (
                        <span className="font-mono font-medium tabular-nums text-foreground">
                          {typeof item.value === "number"
                            ? item.value.toLocaleString()
                            : String(item.value)}
                        </span>
                      )}
                    </div>
                  </>
                )}
              </div>
            )
          })}
        </div>
      </div>
    )
  }
)
ChartTooltipContent.displayName = "ChartTooltip"

const ChartLegend = RechartsPrimitive.Legend

interface ChartLegendContentProps extends React.HTMLAttributes<HTMLDivElement> {
  hideIcon?: boolean
  nameKey?: string
  payload?: unknown
  verticalAlign?: "top" | "middle" | "bottom"
}

const ChartLegendContent = React.forwardRef<HTMLDivElement, ChartLegendContentProps>(
  ({ className, hideIcon = false, payload, verticalAlign = "bottom", nameKey }, ref) => {
    const { config } = useChart()
    const items = Array.isArray(payload)
      ? (payload as Array<{ value?: string; color?: string; dataKey?: string; type?: string }>)
      : []
    if (items.length === 0) return null
    return (
      <div
        ref={ref}
        className={cn(
          "flex items-center justify-center gap-4",
          verticalAlign === "top" ? "pb-3" : "pt-3",
          className
        )}
      >
        {items.map((item) => {
          const key = `${nameKey || item.dataKey || "value"}`
          const itemConfig = getPayloadConfig(config, item, key)
          return (
            <div
              key={item.value ?? key}
              className="flex items-center gap-1.5 [&>svg]:h-3 [&>svg]:w-3 [&>svg]:text-muted-foreground"
            >
              {itemConfig?.icon && !hideIcon ? (
                <itemConfig.icon />
              ) : (
                <div
                  className="h-2 w-2 shrink-0 rounded-[2px]"
                  style={{ backgroundColor: item.color }}
                />
              )}
              {itemConfig?.label}
            </div>
          )
        })}
      </div>
    )
  }
)
ChartLegendContent.displayName = "ChartLegend"

function getPayloadConfig(config: ChartConfig, payload: unknown, key: string) {
  if (typeof payload !== "object" || payload === null) return undefined
  const p = payload as { payload?: Record<string, unknown> } & Record<string, unknown>
  const inner = p.payload && typeof p.payload === "object" ? p.payload : undefined

  let configLabelKey: string = key
  if (key in p && typeof p[key] === "string") {
    configLabelKey = p[key] as string
  } else if (inner && key in inner && typeof inner[key] === "string") {
    configLabelKey = inner[key] as string
  }
  return configLabelKey in config ? config[configLabelKey] : config[key as keyof typeof config]
}

export {
  ChartContainer,
  ChartTooltip,
  ChartTooltipContent,
  ChartLegend,
  ChartLegendContent,
  ChartStyle,
}
