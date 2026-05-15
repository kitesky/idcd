"use client"

import * as React from "react"
import {
  flexRender,
  getCoreRowModel,
  useReactTable,
  type ColumnDef,
  type SortingState,
} from "@tanstack/react-table"
import { ChevronUp, ChevronDown, ChevronsUpDown, ChevronLeft, ChevronRight } from "lucide-react"
import { Button } from "./button"
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from "./select"
import { Table, TableBody, TableCell, TableHead, TableHeader, TableRow } from "./table"
import { cn } from "@/lib/utils"

declare module "@tanstack/react-table" {
  // eslint-disable-next-line @typescript-eslint/no-unused-vars
  interface ColumnMeta<TData, TValue> {
    headerClassName?: string
    cellClassName?: string
  }
}

// ─── Types ──────────────────────────────────────────────────────────────────

export interface DataTablePagination {
  page: number
  pageSize: number
  total: number
}

export interface DataTableSort {
  column: string
  direction: "asc" | "desc"
}

interface DataTableProps<TData> {
  columns: ColumnDef<TData>[]
  data: TData[]
  pagination: DataTablePagination
  sort?: DataTableSort
  loading?: boolean
  onPageChange: (page: number) => void
  onPageSizeChange?: (size: number) => void
  onSortChange?: (sort: DataTableSort | undefined) => void
  emptyMessage?: string
}

// ─── Sort header button ──────────────────────────────────────────────────────

function SortIcon({ direction }: { direction?: "asc" | "desc" }) {
  if (direction === "asc") return <ChevronUp className="ml-1 h-3.5 w-3.5 shrink-0" />
  if (direction === "desc") return <ChevronDown className="ml-1 h-3.5 w-3.5 shrink-0" />
  return <ChevronsUpDown className="ml-1 h-3.5 w-3.5 shrink-0 opacity-40" />
}

// ─── DataTable ───────────────────────────────────────────────────────────────

export function DataTable<TData>({
  columns,
  data,
  pagination,
  sort,
  loading = false,
  onPageChange,
  onPageSizeChange,
  onSortChange,
  emptyMessage = "暂无数据",
}: DataTableProps<TData>) {
  const { page, pageSize, total } = pagination
  const pageCount = Math.max(1, Math.ceil(total / pageSize))

  const [sorting, setSorting] = React.useState<SortingState>(
    sort ? [{ id: sort.column, desc: sort.direction === "desc" }] : []
  )

  const table = useReactTable({
    data,
    columns,
    getCoreRowModel: getCoreRowModel(),
    manualPagination: true,
    manualSorting: true,
    pageCount,
    state: { sorting },
    onSortingChange: (updater) => {
      const next = typeof updater === "function" ? updater(sorting) : updater
      setSorting(next)
      if (onSortChange) {
        if (next.length === 0) {
          onSortChange(undefined)
        } else {
          onSortChange({ column: next[0]!.id, direction: next[0]!.desc ? "desc" : "asc" })
        }
      }
    },
  })

  return (
    <div className="space-y-3">
      {/* Table */}
      <div className={cn(loading && "opacity-60 pointer-events-none")}>
        <Table>
          <TableHeader>
            {table.getHeaderGroups().map((headerGroup) => (
              <TableRow key={headerGroup.id}>
                {headerGroup.headers.map((header) => {
                  const canSort = header.column.getCanSort()
                  const sorted = header.column.getIsSorted()
                  return (
                    <TableHead key={header.id} className={header.column.columnDef.meta?.headerClassName as string}>
                      {header.isPlaceholder ? null : canSort ? (
                        <button
                          className="flex items-center font-medium hover:text-foreground transition-colors"
                          onClick={header.column.getToggleSortingHandler()}
                        >
                          {flexRender(header.column.columnDef.header, header.getContext())}
                          <SortIcon direction={sorted || undefined} />
                        </button>
                      ) : (
                        flexRender(header.column.columnDef.header, header.getContext())
                      )}
                    </TableHead>
                  )
                })}
              </TableRow>
            ))}
          </TableHeader>
          <TableBody>
            {table.getRowModel().rows.length === 0 ? (
              <TableRow>
                <TableCell colSpan={columns.length} className="text-center py-10 text-muted-foreground text-sm">
                  {loading ? "加载中…" : emptyMessage}
                </TableCell>
              </TableRow>
            ) : (
              table.getRowModel().rows.map((row) => (
                <TableRow key={row.id}>
                  {row.getVisibleCells().map((cell) => (
                    <TableCell key={cell.id} className={cell.column.columnDef.meta?.cellClassName as string}>
                      {flexRender(cell.column.columnDef.cell, cell.getContext())}
                    </TableCell>
                  ))}
                </TableRow>
              ))
            )}
          </TableBody>
        </Table>
      </div>

      {/* Pagination */}
      <div className="flex flex-col gap-3 sm:flex-row sm:items-center sm:justify-between text-sm text-muted-foreground px-1">
        <div className="flex items-center gap-2 shrink-0">
          <span>共 {total} 条</span>
          {onPageSizeChange && (
            <Select value={String(pageSize)} onValueChange={(v) => { onPageSizeChange(Number(v)); onPageChange(1) }}>
              <SelectTrigger className="h-7 w-[70px] text-xs">
                <SelectValue />
              </SelectTrigger>
              <SelectContent>
                {[10, 20, 50, 100].map((s) => (
                  <SelectItem key={s} value={String(s)}>{s} 条</SelectItem>
                ))}
              </SelectContent>
            </Select>
          )}
        </div>
        <div className="flex items-center gap-1.5">
          <Button
            variant="outline"
            size="icon"
            className="h-7 w-7"
            disabled={page <= 1 || loading}
            onClick={() => onPageChange(page - 1)}
          >
            <ChevronLeft className="h-4 w-4" />
          </Button>
          <span className="px-2 tabular-nums">
            {page} / {pageCount}
          </span>
          <Button
            variant="outline"
            size="icon"
            className="h-7 w-7"
            disabled={page >= pageCount || loading}
            onClick={() => onPageChange(page + 1)}
          >
            <ChevronRight className="h-4 w-4" />
          </Button>
        </div>
      </div>
    </div>
  )
}
