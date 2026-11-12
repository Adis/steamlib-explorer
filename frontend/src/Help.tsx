/** Help page: why Steam may not share a library, and the two ways to fix it. */
export default function Help() {
  return (
    <article className="mx-auto max-w-3xl space-y-8 text-[15px] leading-relaxed">
      <header>
        <a href="#" className="text-sm text-link hover:text-link-hover">
          ← Back to your library
        </a>
        <h2 className="mt-3 text-2xl font-semibold tracking-tight">Help</h2>
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
            Put it in the <code>.env</code> file next to the app as{" "}
            <code>STEAM_API_KEY=…</code>.
          </li>
          <li>
            Restart the app (<code>make dev</code>), sign in with the same Steam
            account the key belongs to and press <strong>Sync library</strong>.
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
    </article>
  );
}

function Section({ title, children }: { title: string; children: React.ReactNode }) {
  return (
    <section className="space-y-3 rounded-xl border border-line bg-surface px-6 py-5">
      <h3 className="text-lg font-semibold tracking-tight">{title}</h3>
      {children}
    </section>
  );
}

function ExtLink({ href, children }: { href: string; children: React.ReactNode }) {
  return (
    <a href={href} target="_blank" rel="noreferrer" className="text-link hover:text-link-hover">
      {children}
    </a>
  );
}
