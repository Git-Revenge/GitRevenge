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

const features = [
  {
    title: 'A graph for big histories',
    body: 'History loads just ahead of where you scroll, not all at once. On the Linux kernel the aim is a first screen in under a second and 60fps scrolling.',
  },
  {
    title: 'Your git, not ours',
    body: 'It drives the git you already have, with your config, hooks, credential helpers and SSH agent. Anything you do in a terminal shows up straight away.',
  },
  {
    title: 'Everyday git, with undo',
    body: 'Stage files, hunks or single lines. Commit, stash, branch, merge, rebase, cherry-pick, reset and revert, then undo it if you change your mind.',
  },
  {
    title: 'History tools',
    body: 'Interactive rebase, a 3-pane conflict tool, file history, blame, the reflog, git notes and signature badges.',
  },
  {
    title: 'Previews, not just diffs',
    body: 'Markdown, images, CSV, JSON, YAML, TOML, LaTeX, Mermaid and PDF files can be shown rendered, next to the usual unified or split diff.',
  },
  {
    title: 'Nothing leaves your machine',
    body: 'No accounts, no cloud sync, no telemetry and no AI. The one request it makes on its own is an update check, and you can turn it off.',
  },
];

const docLinks = [
  {
    title: 'Install',
    desc: 'Windows and macOS, and what you need first.',
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
    body: 'Drop, squash and fixup in an editor, not a text file.',
    to: '/docs/walkthroughs/tidy-a-branch',
  },
  {
    name: 'conflict',
    title: 'Resolve a conflict',
    body: 'Ours, theirs and the result side by side, then Continue.',
    to: '/docs/walkthroughs/resolve-a-conflict',
  },
];

function Hero() {
  const logo = useBaseUrl('/img/logo.svg');
  return (
    <section className="hero-banner">
      <img src={logo} alt="" width="88" height="88" style={{ marginBottom: '1.25rem' }} />
      <div className="hero-eyebrow">no cloud, no AI, just git</div>
      <h1 className="hero-title">
        Git<br />
        <em className="hero-title-em">Revenge.</em>
      </h1>
      <p className="hero-subtitle">
        A desktop git client for Windows and macOS that stays fast on huge repositories. It
        drives the git you already have, and keeps everything on your machine.
      </p>
      <div className="hero-buttons">
        <Link className="hero-button-primary" to="/docs/guide">
          Get started
        </Link>
        <Link className="hero-button-secondary" to="https://github.com/Git-Revenge/GitRevenge/releases/latest">
          Download
        </Link>
      </div>
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

function DemosSection() {
  return (
    <section className="demos-section">
      <div className="features-header">
        <div className="features-eyebrow">see it work</div>
        <h2 className="features-heading">
          The hard parts of git, <em className="features-heading-em">made visible.</em>
        </h2>
      </div>
      <div className="demo-grid">
        {demos.map(({ name, title, body, to }) => (
          <Link key={name} className="demo-card" to={to}>
            <ThemedImage className="demo-card-shot" alt={title} sources={shot(name)} />
            <p className="feature-card-title">{title}</p>
            <p className="feature-card-body">{body}</p>
          </Link>
        ))}
      </div>
    </section>
  );
}

function QuickInstall() {
  return (
    <section className="quick-install-section">
      <div className="quick-install-block">
        <div className="quick-install-label">Big repository?</div>
        <pre className="quick-install-pre">{
`# Git Revenge needs git 2.40 or newer
git --version

# A commit-graph file makes the first screen much faster
git commit-graph write --reachable

# ...and this keeps it up to date on every fetch
git config fetch.writeCommitGraph true`
        }</pre>
      </div>
    </section>
  );
}

function FeaturesSection() {
  return (
    <section className="features-section">
      <div className="features-header">
        <div className="features-eyebrow">what it does</div>
        <h2 className="features-heading">
          All of git, <em className="features-heading-em">none of the waiting.</em>
        </h2>
        <p className="features-subtitle">
          Built for the repositories other clients give up on, and comfortable on small ones too.
        </p>
      </div>
      <div className="feature-grid">
        {features.map(({ title, body }, i) => (
          <div key={title} className="feature-card">
            <div
              className="feature-card-icon"
              aria-hidden="true"
              style={{ fontFamily: "'JetBrains Mono', monospace", fontSize: '1rem', color: 'var(--dr-rose)' }}
            >
              {String(i + 1).padStart(2, '0')}
            </div>
            <p className="feature-card-title">{title}</p>
            <p className="feature-card-body">{body}</p>
          </div>
        ))}
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
    </section>
  );
}

export default function Home() {
  const { siteConfig } = useDocusaurusContext();
  return (
    <Layout title="A fast, local-only git client" description={siteConfig.tagline}>
      <main>
        <Hero />
        <Showcase />
        <FeaturesSection />
        <DemosSection />
        <QuickInstall />
        <DocLinksSection />
      </main>
    </Layout>
  );
}
