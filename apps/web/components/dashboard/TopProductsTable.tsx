"use client";

import {
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
} from "@/components/ui/table";
import { formatCents } from "./MetricCard";
import { useState } from "react";

interface ProductStat {
  productId: string;
  title: string;
  revenueCents: number;
  unitsSold: number;
}

type SortKey = "revenueCents" | "unitsSold";

interface TopProductsTableProps {
  products: ProductStat[];
  title?: string;
}

export function TopProductsTable({
  products,
  title = "Top Products",
}: TopProductsTableProps) {
  const [sortKey, setSortKey] = useState<SortKey>("revenueCents");

  const sorted = [...products].sort((a, b) => b[sortKey] - a[sortKey]);

  return (
    <div>
      <div className="mb-2 flex items-center justify-between">
        <h3 className="text-sm font-semibold">{title}</h3>
        <div className="flex gap-1 text-xs">
          <button
            className={`rounded px-2 py-0.5 ${sortKey === "revenueCents" ? "bg-primary text-primary-foreground" : "text-muted-foreground hover:text-foreground"}`}
            onClick={() => setSortKey("revenueCents")}
            aria-pressed={sortKey === "revenueCents"}
          >
            Revenue
          </button>
          <button
            className={`rounded px-2 py-0.5 ${sortKey === "unitsSold" ? "bg-primary text-primary-foreground" : "text-muted-foreground hover:text-foreground"}`}
            onClick={() => setSortKey("unitsSold")}
            aria-pressed={sortKey === "unitsSold"}
          >
            Units
          </button>
        </div>
      </div>
      <Table>
        <TableHeader>
          <TableRow>
            <TableHead>Product</TableHead>
            <TableHead className="text-right">Revenue</TableHead>
            <TableHead className="text-right">Units</TableHead>
          </TableRow>
        </TableHeader>
        <TableBody>
          {sorted.length === 0 ? (
            <TableRow>
              <TableCell colSpan={3} className="text-center text-muted-foreground">
                No data
              </TableCell>
            </TableRow>
          ) : (
            sorted.map((p) => (
              <TableRow key={p.productId}>
                <TableCell className="max-w-[180px] truncate font-medium">
                  {p.title}
                </TableCell>
                <TableCell className="text-right tabular-nums">
                  {formatCents(p.revenueCents)}
                </TableCell>
                <TableCell className="text-right tabular-nums">
                  {p.unitsSold.toLocaleString()}
                </TableCell>
              </TableRow>
            ))
          )}
        </TableBody>
      </Table>
    </div>
  );
}
