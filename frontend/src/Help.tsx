import { useEffect } from "react";

/** Link to the Help page's section about AI assistants. */
export const AI_HELP_HREF = "#help/ai-assistants";

/**
 * Help page: why Steam may not share a library and the two ways to fix it,
 * and how to let an AI assistant read the library. "#help/<id>" opens it at
 * the section with that id.
 */
export default function Help({ signedIn }: { signedIn: boolean }) {
  useEffect(() => {
    const id = window.location.hash.split("/")[1];
    if (!id) return;
    // After the app has moved focus to the top of the new page.
    const t = setTimeout(() => document.getElementById(id)?.scrollIntoView({ block: "start" }), 0);
    return () => clearTimeout(t);
  }, []);
  return (
    <article className="mx-auto max-w-3xl space-y-8 text-[0.9375rem] leading-relaxed">
      <header>
        <a href="#" className="text-sm text-link hover:text-link-hover">
          {signedIn ? "← Back to your library" : "← Back to sign in"}
        </a>
        <h2 className="heading mt-3 text-2xl">Help</h2>
      </header>

      <Section title="Why can't the app read my library?">
        <p>
          Store details such as genres, reviews and prices are public, so the app
          can fetch them for any game. Your list of owned games and your playtime
          are different: Steam shares them only in one of two cases.
        </p>
        <ol className="list-decimal space-y-1 pl-6">
          <li>
            Your profile's <strong>Game details</strong> are <strong>Public</strong>, or
          </li>
          <li>
            the app runs with <strong>your own Steam Web API key</strong>.
          </li>
        </ol>
        <p>
          If neither is true, Steam answers with an empty library and the sync
          stops with a message about it. Signing in through Steam doesn't change
          this: sign-in only tells the app who you are, it gives no access to
          your library. <em>Friends only</em> counts as private here.
        </p>
      </Section>

      <Section title="Option 1: make your game details public">
        <p>This works for everyone who signs in to the app.</p>
        <ol className="list-decimal space-y-1 pl-6">
          <li>
            In Steam, open your profile and choose <strong>Edit Profile</strong>,
            then <strong>Privacy Settings</strong>. On the web it's{" "}
            <ExtLink href="https://steamcommunity.com/my/edit/settings">
              steamcommunity.com/my/edit/settings
            </ExtLink>
            .
          </li>
          <li>
            Set <strong>My profile</strong> to <strong>Public</strong>. Steam only
            lets game details be public when the profile is.
          </li>
          <li>
            Set <strong>Game details</strong> to <strong>Public</strong>.
          </li>
          <li>
            To see your playtime in the app, leave{" "}
            <strong>Always keep my total playtime private</strong> unticked.
          </li>
          <li>
            Wait a minute for Steam to pick up the change, then press{" "}
            <strong>Sync library</strong>.
          </li>
        </ol>
        <p className="text-muted">
          Anyone can then see which games you own and how long you played them.
        </p>
      </Section>

      <Section title="Option 2: use your own Steam API key">
        <p>
          The app reads libraries with the Steam Web API key it was started with.
          Steam always shows a key's owner their own library, whatever their
          privacy settings. So if you run your own copy of the app with your own
          key, your profile and game details can stay private, even on{" "}
          <em>Friends only</em> or <em>Private</em>.
        </p>
        <ol className="list-decimal space-y-1 pl-6">
          <li>
            Get a key at{" "}
            <ExtLink href="https://steamcommunity.com/dev/apikey">
              steamcommunity.com/dev/apikey
            </ExtLink>
            . Any domain name works, for example <code>localhost</code>.
          </li>
          <li>
            The first time the app starts without a key, it asks for one and saves
            it in your computer's keychain (Windows Credential Manager, or GNOME
            Keyring or KWallet on Linux). Without a keychain it goes in{" "}
            <code>settings.json</code> in the app's data folder instead. Running from
            source, you can instead put <code>STEAM_API_KEY=…</code> in the{" "}
            <code>.env</code> file, which wins over the saved key, and restart the
            app (<code>make dev</code>).
          </li>
          <li>
            Sign in with the same Steam account the key belongs to and press{" "}
            <strong>Sync library</strong>.
          </li>
        </ol>
        <p className="text-muted">
          This only helps the account that owns the key. Anyone else who signs in
          to your copy still needs option 1. Keep your key to yourself: Steam's
          terms don't allow sharing it, so don't give it to someone else's copy of
          the app.
        </p>
      </Section>

      <Section title="Still not working?">
        <p>
          The same message appears when an account owns no games at all. If your
          settings are right and you own games, wait a few minutes and sync
          again: Steam can take a while to apply a privacy change.
        </p>
      </Section>

      <Section id="ai-assistants" title="Ask an AI assistant about your library">
        <p>
          The app can let an AI assistant such as Claude read your library, so you can ask
          about it in plain words. It uses MCP (Model Context Protocol), a standard way for
          assistants to use other programs. The assistant can search your games with the same
          filters as the explorer, look up a game, see your library stats and list other
          games by a developer. It can only read: it can't change anything, start a sync or
          reach your Steam account.
        </p>
        <h4 className="font-semibold">1. Make a token</h4>
        <ol className="list-decimal space-y-1 pl-6">
          <li>
            Sign in, open <a href="#settings" className="text-link hover:text-link-hover">Settings</a>{" "}
            and find <strong>AI assistants (MCP)</strong>.
          </li>
          <li>
            Press <strong>Make token</strong> and copy the token straight away. The app keeps
            only a fingerprint of it, so it can't show it again. If you lose it, press{" "}
            <strong>Replace token</strong> and set up the assistant again.
          </li>
          <li>
            Note the <strong>Server address</strong> shown there, usually{" "}
            <code>http://localhost:8080/mcp</code>.
          </li>
        </ol>

        <h4 className="font-semibold">2. Connect your assistant</h4>
        <p>
          <strong>Claude Code</strong>: right after you make a token, Settings shows a{" "}
          <code>claude mcp add …</code> command. Paste it into a terminal, then start{" "}
          <code>claude</code> and ask away. <code>/mcp</code> in Claude Code shows whether
          it's connected.
        </p>
        <p>
          <strong>Claude Desktop</strong>: its Connectors reach servers on the internet, not
          ones on your computer, so it connects through a small helper,{" "}
          <ExtLink href="https://www.npmjs.com/package/mcp-remote">mcp-remote</ExtLink>, which
          needs <ExtLink href="https://nodejs.org">Node.js</ExtLink>. In Claude Desktop open{" "}
          <strong>Settings</strong>, <strong>Developer</strong>, <strong>Edit Config</strong>,
          add this with your server address and token, and restart Claude Desktop:
        </p>
        <pre className="overflow-x-auto rounded-md border border-control bg-canvas px-3 py-2 font-mono text-[0.8125rem]">{`{
  "mcpServers": {
    "steamlib": {
      "command": "npx",
      "args": ["-y", "mcp-remote", "http://localhost:8080/mcp",
               "--header", "Authorization:\${STEAMLIB_AUTH}"],
      "env": { "STEAMLIB_AUTH": "Bearer slx_your_token" }
    }
  }
}`}</pre>
        <p>
          Other assistants that support MCP over HTTP work too: give them the server address
          and the header <code>Authorization: Bearer &lt;your token&gt;</code>.
        </p>

        <h4 className="font-semibold">3. Ask</h4>
        <ul className="list-disc space-y-1 pl-6">
          <li>"Which Overwhelmingly Positive games have I never played?"</li>
          <li>"Suggest a short strategy game I own for this weekend."</li>
          <li>"What did I play most in the last two weeks?"</li>
          <li>"What's in my library from the last six months?"</li>
          <li>"Which games by the makers of my most played game don't I own?"</li>
          <li>"How many hours have I spent on Steam, and on which genres?"</li>
        </ul>

        <p className="text-muted">
          The app must be running for the assistant to reach it, and answers are as fresh as
          your last <strong>Sync library</strong>. Anyone with your token can read your
          library, so keep it to yourself; <strong>Revoke token</strong> in Settings cuts off
          every assistant using it. Each person who signs in makes their own token, which reads
          only their library.
        </p>
      </Section>
    </article>
  );
}

function Section({ id, title, children }: { id?: string; title: string; children: React.ReactNode }) {
  return (
    <section id={id} className="scroll-mt-4 space-y-3 panel rounded-xl border border-line bg-surface px-6 py-5">
      <h3 className="heading text-lg">{title}</h3>
      {children}
    </section>
  );
}

function ExtLink({ href, children }: { href: string; children: React.ReactNode }) {
  return (
    <a href={href} target="_blank" rel="noreferrer" className="text-link hover:text-link-hover">
      {children}
      <span className="sr-only"> (opens in a new tab)</span>
    </a>
  );
}
