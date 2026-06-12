"use client";

interface ResultTableProps {
  columns: string[];
  rows: (string | null)[][];
}

export function ResultTable({ columns, rows }: ResultTableProps) {
  if (columns.length === 0) return null;

  return (
    <div className="overflow-x-auto max-h-72 overflow-y-auto rounded-md border">
      <table className="w-full text-sm" role="table" aria-label="Query results">
        <thead className="sticky top-0 bg-muted">
          <tr>
            {columns.map((col) => (
              <th
                key={col}
                scope="col"
                className="px-3 py-2 text-left text-xs font-semibold uppercase tracking-wide text-muted-foreground"
              >
                {col}
              </th>
            ))}
          </tr>
        </thead>
        <tbody>
          {rows.length === 0 ? (
            <tr>
              <td
                colSpan={columns.length}
                className="px-3 py-4 text-center text-sm text-muted-foreground"
              >
                No results
              </td>
            </tr>
          ) : (
            rows.map((row, i) => (
              <tr key={i} className="border-t hover:bg-muted/50">
                {row.map((cell, j) => (
                  <td key={j} className="px-3 py-1.5 font-mono text-xs">
                    {cell === null ? (
                      <span className="italic text-muted-foreground">null</span>
                    ) : (
                      cell
                    )}
                  </td>
                ))}
              </tr>
            ))
          )}
        </tbody>
      </table>
    </div>
  );
}
