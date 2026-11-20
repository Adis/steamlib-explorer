import { useState } from "react";
import { useMutation, useQueryClient } from "@tanstack/react-query";
import { KeyIcon } from "./icons.tsx";
import { saveApiKey } from "./api.ts";
import { ErrorBox } from "./ui.tsx";

/** First run: asks for the Steam Web API key the app reads libraries with. */
export default function Setup() {
  const [key, setKey] = useState("");
  const queryClient = useQueryClient();
  const save = useMutation({
    mutationFn: saveApiKey,
    onSuccess: () => queryClient.setQueryData(["setup"], { needs_api_key: false }),
  });

  return (
    <section className="mx-auto mt-16 max-w-xl panel rounded-xl border border-line bg-surface p-10 shadow-xl shadow-black/20">
      <KeyIcon className="mb-5 h-12 w-12 text-accent" />
      <h2 className="page-title text-2xl">One-time setup</h2>
      <p className="mt-2 text-muted">
        Steamlib Explorer reads your library through the Steam Web API, which needs a free key
        tied to your Steam account. It stays on this computer.
      </p>
      <ol className="mt-4 list-decimal space-y-1 pl-6">
        <li>
          Open{" "}
          <a
            href="https://steamcommunity.com/dev/apikey"
            target="_blank"
            rel="noreferrer"
            className="text-link hover:text-link-hover"
          >
            steamcommunity.com/dev/apikey
            <span className="sr-only"> (opens in a new tab)</span>
          </a>{" "}
          and sign in. For the domain name, anything works, for example <code>localhost</code>.
        </li>
        <li>Copy the key Steam shows you and paste it here.</li>
      </ol>
      <form
        className="mt-6 space-y-3"
        onSubmit={(e) => {
          e.preventDefault();
          save.mutate(key.trim());
        }}
      >
        <label htmlFor="api-key" className="block text-sm font-medium">
          Steam Web API key
        </label>
        <input
          id="api-key"
          value={key}
          onChange={(e) => setKey(e.target.value)}
          autoComplete="off"
          spellCheck={false}
          placeholder="32 letters and digits"
          className="w-full rounded-md border border-control bg-surface px-2.5 py-2 font-mono text-fg placeholder:text-faint transition-colors hover:border-faint focus:border-accent"
        />
        {save.isError && <ErrorBox>{save.error.message}</ErrorBox>}
        <button
          type="submit"
          disabled={save.isPending || key.trim() === ""}
          className="btn-action rounded-md bg-accent px-5 py-2.5 font-medium text-on-accent hover:bg-accent-hover disabled:opacity-50"
        >
          {save.isPending ? "Checking with Steam…" : "Save key"}
        </button>
      </form>
      <p className="mt-6 text-sm text-muted">
        Steam always shows a key's owner their own library, so with your own key your profile can
        stay private. The app keeps the key in your computer's keychain. Keep it to yourself:
        Steam's terms don't allow sharing it.
      </p>
    </section>
  );
}
