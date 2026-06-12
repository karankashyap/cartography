"use client";

import {
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
} from "@/components/ui/table";
import { Badge } from "@/components/ui/badge";

interface VariantStat {
  variantId: string;
  sku?: string | null;
  title: string;
  inventoryQty: number;
  daysSinceLastSale: number;
}

interface DeadStockTableProps {
  variants: VariantStat[];
}

function staleBadge(days: number) {
  if (days >= 999) return { label: "Never sold", variant: "destructive" as const };
  if (days >= 180) return { label: `${days}d`, variant: "destructive" as const };
  if (days >= 90) return { label: `${days}d`, variant: "secondary" as const };
  return { label: `${days}d`, variant: "outline" as const };
}

export function DeadStockTable({ variants }: DeadStockTableProps) {
  return (
    <div>
      <h3 className="mb-2 text-sm font-semibold">Dead Stock</h3>
      <Table>
        <TableHeader>
          <TableRow>
            <TableHead>Product / SKU</TableHead>
            <TableHead className="text-right">Qty</TableHead>
            <TableHead className="text-right">Last Sale</TableHead>
          </TableRow>
        </TableHeader>
        <TableBody>
          {variants.length === 0 ? (
            <TableRow>
              <TableCell colSpan={3} className="text-center text-muted-foreground">
                No dead stock
              </TableCell>
            </TableRow>
          ) : (
            variants.map((v) => {
              const badge = staleBadge(v.daysSinceLastSale);
              return (
                <TableRow key={v.variantId}>
                  <TableCell>
                    <span className="font-medium">{v.title}</span>
                    {v.sku && (
                      <span className="ml-1.5 text-xs text-muted-foreground">
                        {v.sku}
                      </span>
                    )}
                  </TableCell>
                  <TableCell className="text-right tabular-nums">
                    {v.inventoryQty}
                  </TableCell>
                  <TableCell className="text-right">
                    <Badge variant={badge.variant} aria-label={`${badge.label} since last sale`}>
                      {badge.label}
                    </Badge>
                  </TableCell>
                </TableRow>
              );
            })
          )}
        </TableBody>
      </Table>
    </div>
  );
}
