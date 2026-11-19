import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { Check, Copy } from "lucide-react";
import { useState } from "react";
import { createMcpToken, deleteMcpToken, fetchMcpToken } from "./api.ts";
import { AI_HELP_HREF } from "./Help.tsx";
import { ErrorBox, formatDateTime } from "./ui.tsx";

/**
 * Settings section for the MCP server: lets an AI assistant such as Claude
 * read the library with a token made here. Unlike the rest of Settings, the
 * token is kept by the app for the signed-in account.
 */
export default function McpAccess() {
  const queryClient = useQueryClient();
  const status = useQuery({ queryKey: ["mcp-token"], queryFn: fetchMcpToken });
  // The new token, shown until the page is left: it can't be read again.
  const [token, setToken] = useState<string | null>(null);
  const create = useMutation({
    mutationFn: createMcpToken,
    onSuccess: (t) => {
      setToken(t.token ?? null);
      queryClient.setQueryData(["mcp-token"], { exists: true, created_at: t.created_at });
    },
  });
  const revoke = useMutation({
    mutationFn: deleteMcpToken,
    onSuccess: () => {
      setToken(null);
      queryClient.setQueryData(["mcp-token"], { exists: false, created_at: null });
    },
  });

  const url = `${window.location.origin}/mcp`;
  const exists = status.data?.exists ?? false;
  const busy = create.isPending || revoke.isPending;

  return (
    <div>
      <h3 className="font-semibold tracking-tight">AI assistants (MCP)</h3>
      <p className="mt-0.5 text-sm text-muted">
        Let an AI assistant such as Claude read your library through MCP: search your games, look
        up a game, see library stats and other games by a developer. It can only read; it can't
        change anything or start a sync. The token is kept by the app for your account, not in
        this browser.{" "}
        <a href={AI_HELP_HREF} className="text-link hover:text-link-hover">
          How to connect Claude
        </a>
      </p>

      <dl className="mt-4 space-y-3 text-sm">
        <div>
          <dt className="font-medium">Server address</dt>
          <dd className="mt-1">
            <CopyField value={url} label="server address" />
          </dd>
        </div>
        <div>
          <dt className="font-medium">Token</dt>
          <dd className="mt-1 text-muted">
            {status.isPending
              ? "Checking…"
              : exists && status.data?.created_at
                ? `Made ${formatDateTime(status.data.created_at)}. It was shown once; make a new one if you lost it.`
                : "None yet."}
          </dd>
        </div>
      </dl>

      {token && (
        <div className="mt-4 space-y-3 rounded-lg border border-accent p-4 text-sm">
          <p className="font-medium">Copy your token now: it won't be shown again.</p>
          <CopyField value={token} label="token" />
          <p className="text-muted">For Claude Code, run this in a terminal:</p>
          <CopyField
            value={`claude mcp add --transport http steamlib ${url} --header "Authorization: Bearer ${token}"`}
            label="Claude Code command"
            multiline
          />
          <p className="text-muted">Anyone with this token can read your library. Keep it to yourself.</p>
        </div>
      )}

      {status.isError && <ErrorBox className="mt-4">{status.error.message}</ErrorBox>}
      {create.isError && <ErrorBox className="mt-4">{create.error.message}</ErrorBox>}
      {revoke.isError && <ErrorBox className="mt-4">{revoke.error.message}</ErrorBox>}

      <div className="mt-4 flex flex-wrap gap-2">
        <button
          onClick={() => create.mutate()}
          disabled={busy || status.isPending}
          title={exists ? "Assistants using the old token lose access." : undefined}
          className="rounded-md bg-accent px-4 py-2 text-sm font-medium text-on-accent hover:bg-accent-hover disabled:opacity-50"
        >
          {create.isPending ? "Making…" : exists ? "Replace token" : "Make token"}
        </button>
        {exists && (
          <button
            onClick={() => revoke.mutate()}
            disabled={busy}
            className="rounded-md border border-line-strong px-4 py-2 text-sm font-medium text-fg hover:bg-raised disabled:opacity-50"
          >
            {revoke.isPending ? "Revoking…" : "Revoke token"}
          </button>
        )}
      </div>
    </div>
  );
}

/** Read-only text with a copy button. */
function CopyField({ value, label, multiline = false }: { value: string; label: string; multiline?: boolean }) {
  const [copied, setCopied] = useState(false);
  const copy = async () => {
    try {
      await navigator.clipboard.writeText(value);
      setCopied(true);
      setTimeout(() => setCopied(false), 2000);
    } catch {
      // No clipboard (e.g. plain http on another computer): the text can still be selected.
    }
  };
  return (
    <div className="flex items-start gap-2">
      <code
        className={`min-w-0 flex-1 rounded-md border border-control bg-canvas px-2.5 py-1.5 font-mono text-[0.8125rem] text-fg select-all ${multiline ? "whitespace-pre-wrap [overflow-wrap:anywhere]" : "truncate"}`}
      >
        {value}
      </code>
      <button
        onClick={copy}
        aria-label={`Copy ${label}`}
        className="inline-flex shrink-0 items-center gap-1.5 rounded-md border border-line-strong px-2.5 py-1.5 text-sm text-fg hover:bg-raised"
      >
        {copied ? <Check aria-hidden className="h-4 w-4 text-accent" /> : <Copy aria-hidden className="h-4 w-4" />}
        <span aria-live="polite">{copied ? "Copied" : "Copy"}</span>
      </button>
    </div>
  );
}
