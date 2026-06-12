"use client";

import { useRef, useState } from "react";
import { useMutation, useSubscription } from "@urql/next";

const IMPORT_STORE_MUTATION = `
  mutation ImportStore($filename: String!, $platform: Platform!) {
    importStore(filename: $filename, platform: $platform) {
      id
      state
      filename
      platform
    }
  }
`;

const IMPORT_PROGRESS_SUBSCRIPTION = `
  subscription ImportProgress($jobId: ID!) {
    importProgress(jobId: $jobId) {
      id
      state
      rowsParsed
      rowsSkipped
      error
    }
  }
`;

const UPLOAD_URL =
  process.env.NEXT_PUBLIC_API_URL?.replace("/query", "/upload") ??
  "http://localhost:8080/upload";

type JobState = "PENDING" | "RUNNING" | "DONE" | "FAILED";

interface ImportJob {
  id: string;
  state: JobState;
  rowsParsed?: number;
  rowsSkipped?: number;
  error?: string;
}

interface ImportButtonProps {
  onComplete?: () => void;
}

export function ImportButton({ onComplete }: ImportButtonProps) {
  const fileInputRef = useRef<HTMLInputElement>(null);
  const [jobId, setJobId] = useState<string | null>(null);
  const [status, setStatus] = useState<string>("");
  const [uploading, setUploading] = useState(false);

  const [, importStore] = useMutation(IMPORT_STORE_MUTATION);

  useSubscription(
    {
      query: IMPORT_PROGRESS_SUBSCRIPTION,
      variables: { jobId: jobId ?? "" },
      pause: !jobId,
    },
    (_prev: ImportJob | undefined, data: { importProgress: ImportJob }): ImportJob => {
      const job = data?.importProgress;
      if (!job) return _prev ?? { id: "", state: "PENDING" };
      if (job.state === "DONE") {
        setStatus(`Import complete: ${job.rowsParsed} rows`);
        setJobId(null);
        onComplete?.();
      } else if (job.state === "FAILED") {
        setStatus(`Import failed: ${job.error ?? "unknown error"}`);
        setJobId(null);
      } else if (job.state === "RUNNING") {
        setStatus(`Importing… ${job.rowsParsed ?? 0} rows processed`);
      }
      return job;
    }
  );

  async function handleFileChange(e: React.ChangeEvent<HTMLInputElement>) {
    const file = e.target.files?.[0];
    if (!file) return;

    setUploading(true);
    setStatus("Uploading file…");

    try {
      const form = new FormData();
      form.append("file", file);
      const res = await fetch(UPLOAD_URL, { method: "POST", body: form });
      if (!res.ok) throw new Error(await res.text());

      const { filename } = (await res.json()) as { filename: string };
      setStatus("Starting import…");

      const result = await importStore({
        filename,
        platform: "SHOPIFY",
      });

      if (result.error) throw new Error(result.error.message);

      const id = result.data?.importStore?.id;
      if (id) {
        setJobId(id);
        setStatus("Import queued…");
      }
    } catch (err) {
      setStatus(`Error: ${err instanceof Error ? err.message : String(err)}`);
    } finally {
      setUploading(false);
      if (fileInputRef.current) fileInputRef.current.value = "";
    }
  }

  return (
    <div className="flex items-center gap-3">
      <label
        htmlFor="csv-upload"
        className={`inline-flex cursor-pointer items-center gap-2 rounded-md border px-4 py-2 text-sm font-medium transition-colors hover:bg-accent focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring ${uploading || jobId ? "pointer-events-none opacity-50" : ""}`}
        aria-label="Import Shopify CSV"
      >
        {uploading ? "Uploading…" : "Import CSV"}
      </label>
      <input
        id="csv-upload"
        ref={fileInputRef}
        type="file"
        accept=".csv"
        className="sr-only"
        onChange={handleFileChange}
        aria-label="Upload Shopify CSV file"
        disabled={uploading || !!jobId}
      />
      {status && (
        <span
          className="text-sm text-muted-foreground"
          role="status"
          aria-live="polite"
        >
          {status}
        </span>
      )}
    </div>
  );
}
