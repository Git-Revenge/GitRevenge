import React from 'react';
import Link from '@docusaurus/Link';
import useBaseUrl from '@docusaurus/useBaseUrl';
import useDocusaurusContext from '@docusaurus/useDocusaurusContext';
import Layout from '@theme/Layout';
import ThemedImage from '@theme/ThemedImage';

// Screenshots live with the docs (docs/img/), in a light and a dark version; see
// scripts/screenshots.mjs.
const shot = (name) => ({
  light: require(`../../../docs/img/${name}-light.webp`).default,
  dark: require(`../../../docs/img/${name}-dark.webp`).default,
});

const RELEASES = 'https://github.com/Git-Revenge/GitRevenge/releases/latest';
const REPO = 'https://github.com/Git-Revenge/GitRevenge';
const SPONSOR = 'https://github.com/sponsors/risadams';

// Why someone would pick this client over another. Each is a claim the app has to keep, so
// change the words here only when the app changes.
const reasons = [
  {
    title: 'It’s free. All of it.',
    body: (
      <>
        There’s no paid tier, no trial that runs out, and no licence to buy before you can open a
        private repository or use it at work. Every feature is in the one download. The source is
        on <Link to={REPO}>GitHub</Link> under Apache-2.0, so it stays that way.
      </>
    ),
  },
  {
    title: 'Your code is nobody else’s business.',
    body: (
      <>
        You never make an account or sign in. There’s no telemetry, no cloud sync, and no AI
        reading your diffs. The app makes one request on its own: at startup it asks GitHub
        whether a newer release exists. <Link to="/docs/guide/settings">Settings</Link> turns that
        off.
      </>
    ),
  },
  {
    title: 'It stays quick when the repository is huge.',
    body: (
      <>
        The graph reads history just ahead of where you’re looking instead of all of it. On the
        Linux kernel, with 1.5 million commits, the first screen arrives in under half a second
        and scrolling stays smooth all the way down.
      </>
    ),
  },
  {
    title: 'It’s your git, not a copy of it.',
    body: (
      <>
        Git Revenge drives the <code>git</code> you already installed, so your config, hooks,
        aliases, signing keys, credential helpers and SSH agent all work as they do in a
        terminal. Commit in a terminal and the app shows it within a second. Stop using the app
        tomorrow and nothing about your repositories has changed.
      </>
    ),
  },
  {
    title: 'It tells you first, and lets you take it back.',
    body: (
      <>
        A merge or rebase says which files will conflict before it runs. Commits, merges,
        rebases, resets and pulls can be undone with one key. Anything that throws work away
        asks, and says what you’d lose.
      </>
    ),
  },
  {
    title: 'It’s small.',
    body: (
      <>
        The download is under 20 MB, because the app uses the web view your system already has
        instead of shipping a browser of its own. Everything it remembers is one settings file
        and your own git config.
      </>
    ),
  },
];

const trades = [
  ['A subscription for private repositories, or for the “pro” features', 'Free, for everything'],
  ['An account before you can open a repository', 'No account, ever'],
  ['Usage analytics, on unless you find the switch', 'No telemetry to switch off'],
  ['A bundled git with settings of its own', 'The git you already use'],
  ['A long wait on a big monorepo', 'A first screen in under a second'],
  ['An assistant that sends your code somewhere', 'No AI'],
];

const features = [
  {
    title: 'A graph you can read',
    body: 'Commits in date order with your main branch as a spine, a line for each day, bot and docs commits folded away, and a ruler at the edge that shows where the branches, tags and search matches are.',
    to: '/docs/guide/graph',
  },
  {
    title: 'Everyday git',
    body: 'Stage files, hunks or single lines. Commit, amend, stash, branch, tag, merge, rebase, cherry-pick, reset and revert. Signed commits if your config signs them.',
    to: '/docs/guide/changes',
  },
  {
    title: 'The hard parts',
    body: 'An interactive rebase editor that shows the branch as it will be. A conflict tool built around the result. Bisect, blame, file history, the reflog and git notes.',
    to: '/docs/guide/branches',
  },
  {
    title: 'Branch care',
    body: 'Branches have kinds, so release and bot branches stop crowding your work. Merged branches clean themselves up after a pull, squash merges included, and stacked branches restack in one click.',
    to: '/docs/guide/branch-care',
  },
  {
    title: 'Previews, not only diffs',
    body: 'Markdown, images, CSV, JSON, YAML, TOML, LaTeX, Mermaid and PDF files can be shown rendered, beside the usual unified or split diff.',
    to: '/docs/guide/history',
  },
  {
    title: 'Many repositories',
    body: 'Repositories open in tabs that come back when you restart. Put related ones in a group and fetch or pull them all from one dashboard. Worktrees are listed and open as tabs.',
    to: '/docs/guide/repositories',
  },
  {
    title: 'Editors for git’s own files',
    body: 'See every ignore rule that applies and which one decides a path. Edit git config with a line on what each setting does, where a value comes from, and a backup on every save.',
    to: '/docs/guide/ignoring',
  },
  {
    title: 'Insights for sprint reviews',
    body: 'What shipped in a window, which branches are stuck or would conflict, and where the hotspots are. It’s worked out from the repository alone, so anyone gets the same numbers.',
    to: '/docs/guide/insights',
  },
  {
    title: 'Keyboard first, when you want',
    body: 'A command palette holds every command. Common ones have shortcuts you can change, and menus, lists and dialogs all work without a mouse.',
    to: '/docs/guide/settings',
  },
];

const docLinks = [
  {
    title: 'Install',
    desc: 'Windows, macOS and Linux, and what you need first.',
    to: '/docs/guide/install',
    accent: 'rose',
  },
  {
    title: 'Take the tour',
    desc: 'Every part of the window, in one picture.',
    to: '/docs/guide/tour',
    accent: 'navy',
  },
  {
    title: 'Walkthroughs',
    desc: 'Commit, rebase, resolve, undo, step by step.',
    to: '/docs/walkthroughs',
    accent: 'plum',
  },
  {
    title: 'Shortcuts',
    desc: 'Settings, and every key you can press or rebind.',
    to: '/docs/guide/settings',
    accent: 'amber',
  },
];

const demos = [
  {
    name: 'stage-and-commit',
    title: 'Commit part of a file',
    body: 'Stage a hunk or a few lines, write a message, Ctrl+Enter.',
    to: '/docs/walkthroughs/first-commit',
  },
  {
    name: 'rebase',
    title: 'Tidy a branch',
    body: 'Drop, squash and fold in fixups, and see the result before you run it.',
    to: '/docs/walkthroughs/tidy-a-branch',
  },
  {
    name: 'conflict',
    title: 'Resolve a conflict',
    body: 'Pick a side for each place, or type the answer. No conflict markers.',
    to: '/docs/walkthroughs/resolve-a-conflict',
  },
  {
    name: 'undo',
    title: 'Undo a mistake',
    body: 'Merged the wrong branch? Ctrl+Z puts it back.',
    to: '/docs/walkthroughs/undo-a-mistake',
  },
  {
    name: 'search',
    title: 'Find a commit',
    body: 'Search by message, author, SHA or trailer, through the whole history.',
    to: '/docs/walkthroughs/find-a-commit',
  },
  {
    name: 'bisect',
    title: 'Find what broke it',
    body: 'Answer Good or Bad as it halves the range, down to the one commit.',
    to: '/docs/walkthroughs/find-what-broke',
  },
];

const installs = [
  {
    os: 'macOS',
    how: 'With Homebrew, or the signed and notarized disk image.',
    code: 'brew install --cask git-revenge/tap/gitrevenge',
  },
  {
    os: 'Windows',
    how: 'Run the installer from the latest release. It isn’t signed yet, so SmartScreen asks first.',
    code: 'GitRevenge-<version>-windows-amd64-installer.exe',
  },
  {
    os: 'Linux',
    how: 'From the signed apt and dnf repositories, for Ubuntu, Debian and RHEL 9 and 10. Add one once; the install guide has the commands.',
    code: 'sudo apt install gitrevenge\nsudo dnf install gitrevenge',
  },
];

function Hero() {
  const logo = useBaseUrl('/img/logo.svg');
  return (
    <section className="hero-banner">
      <img src={logo} alt="" width="88" height="88" style={{ marginBottom: '1.25rem' }} />
      <div className="hero-eyebrow">free, private, and fast</div>
      <h1 className="hero-title">
        Git<br />
        <em className="hero-title-em">Revenge.</em>
      </h1>
      <p className="hero-subtitle">
        A desktop git client that costs nothing, never asks you to sign in, and stays quick on
        repositories with a million commits. It runs on your computer, with the git you already
        have.
      </p>
      <div className="hero-buttons">
        <Link className="hero-button-primary" to={RELEASES}>
          Download
        </Link>
        <Link className="hero-button-secondary" to="/docs/guide">
          Read the guide
        </Link>
      </div>
      <ul className="hero-facts">
        <li>Free and open source</li>
        <li>No account</li>
        <li>No telemetry</li>
        <li>Windows, macOS and Linux</li>
      </ul>
    </section>
  );
}

function Showcase() {
  return (
    <section className="showcase-section">
      <ThemedImage
        className="showcase-shot"
        alt="Git Revenge showing the history of a project: branches, a stash and uncommitted changes"
        sources={shot('repository')}
      />
    </section>
  );
}

function WhySection() {
  return (
    <section className="why-section">
      <div className="features-header">
        <div className="features-eyebrow">why this one</div>
        <h2 className="features-heading">
          A git client that <em className="features-heading-em">leaves you alone.</em>
        </h2>
        <p className="features-subtitle">
          Most git clients are good at git. The trouble is what comes with them: a price, a
          sign-in, a cloud, an assistant. Git Revenge is the client without the rest.
        </p>
      </div>
      <ol className="why-list">
        {reasons.map(({ title, body }) => (
          <li key={title} className="why-item">
            <h3 className="why-item-title">{title}</h3>
            <p className="why-item-body">{body}</p>
          </li>
        ))}
      </ol>
      <div className="trade-wrap">
        <table className="trade-table">
          <thead>
            <tr>
              <th scope="col">What you may be used to</th>
              <th scope="col">Git Revenge</th>
            </tr>
          </thead>
          <tbody>
            {trades.map(([theirs, ours]) => (
              <tr key={ours}>
                <td>{theirs}</td>
                <td>{ours}</td>
              </tr>
            ))}
          </tbody>
        </table>
      </div>
    </section>
  );
}

function DemosSection() {
  return (
    <section className="demos-section">
      <div className="features-header">
        <div className="features-eyebrow">see it work</div>
        <h2 className="features-heading">
          The hard parts of git, <em className="features-heading-em">made visible.</em>
        </h2>
        <p className="features-subtitle">
          Each of these is a walkthrough you can follow in a practice repository, without touching
          real work.
        </p>
      </div>
      <div className="demo-grid">
        {demos.map(({ name, title, body, to }) => (
          <Link key={name} className="demo-card" to={to}>
            <ThemedImage className="demo-card-shot" alt="" sources={shot(name)} />
            <p className="feature-card-title">{title}</p>
            <p className="feature-card-body">{body}</p>
          </Link>
        ))}
      </div>
    </section>
  );
}

function FeaturesSection() {
  return (
    <section className="features-section">
      <div className="features-header">
        <div className="features-eyebrow">what’s in it</div>
        <h2 className="features-heading">
          All of git, <em className="features-heading-em">none of the waiting.</em>
        </h2>
        <p className="features-subtitle">
          Built for the repositories other clients give up on, and comfortable on small ones too.
        </p>
      </div>
      <div className="feature-grid">
        {features.map(({ title, body, to }, i) => (
          <Link key={title} className="feature-card feature-card--link" to={to}>
            <div className="feature-card-icon feature-card-number" aria-hidden="true">
              {String(i + 1).padStart(2, '0')}
            </div>
            <p className="feature-card-title">{title}</p>
            <p className="feature-card-body">{body}</p>
          </Link>
        ))}
      </div>
    </section>
  );
}

function NotSection() {
  return (
    <section className="not-section">
      <div className="not-block">
        <div className="features-eyebrow">to be fair</div>
        <h2 className="not-heading">What it doesn’t do</h2>
        <p>
          Git Revenge knows git, and nothing about the site your repository is hosted on. It won’t
          open pull requests, show review comments or move a ticket. <Link to="/docs/guide/settings#links">Smart links</Link>{' '}
          turn a ticket key in a commit message into a link, and that link opens in your browser.
        </p>
        <p>
          It also needs git 2.40 or newer on your machine, because it doesn’t bring its own. If
          you want pull requests inside your git client, or an assistant to write commit messages,
          another client will suit you better.
        </p>
      </div>
    </section>
  );
}

function InstallSection() {
  return (
    <section className="quick-install-section">
      <div className="install-inner">
        <div className="features-header">
          <div className="features-eyebrow">two minutes</div>
          <h2 className="features-heading">Get it</h2>
        </div>
        <div className="install-grid">
          {installs.map(({ os, how, code }) => (
            <div key={os} className="install-card">
              <h3 className="install-card-os">{os}</h3>
              <p className="install-card-how">{how}</p>
              <pre className="install-card-code">{code}</pre>
            </div>
          ))}
        </div>
        <div className="hero-buttons install-buttons">
          <Link className="hero-button-primary" to={RELEASES}>
            Download the latest release
          </Link>
          <Link className="hero-button-secondary" to="/docs/guide/install">
            Installing, in full
          </Link>
        </div>
      </div>
    </section>
  );
}

function DocLinksSection() {
  return (
    <section className="doc-links-section">
      <h2 className="doc-links-heading">Documentation</h2>
      <div className="doc-links-grid">
        {docLinks.map(({ title, desc, to, accent }) => (
          <Link key={title} className={`doc-card doc-card--${accent}`} to={to}>
            <div className="doc-card-title">{title}</div>
            <div className="doc-card-desc">{desc}</div>
          </Link>
        ))}
      </div>
      <p className="sponsor-note">
        Git Revenge is an independent project, and it’s free to use. If it saves you time, you
        can <Link to={SPONSOR}>sponsor it</Link>. Bugs and ideas go to{' '}
        <Link to={`${REPO}/issues`}>the issue tracker</Link>.
      </p>
    </section>
  );
}

export default function Home() {
  const { siteConfig } = useDocusaurusContext();
  return (
    <Layout title="A free, private git client" description={siteConfig.tagline}>
      <main>
        <Hero />
        <Showcase />
        <WhySection />
        <DemosSection />
        <FeaturesSection />
        <NotSection />
        <InstallSection />
        <DocLinksSection />
      </main>
    </Layout>
  );
}
