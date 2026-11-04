import React from 'react';
import Link from '@docusaurus/Link';
import useBaseUrl from '@docusaurus/useBaseUrl';
import { PageMetadata } from '@docusaurus/theme-common';

// The page for a URL that leads nowhere, in place of Docusaurus's, which asks the reader to
// tell the site's owner. It uses the homepage's hero and documentation cards, and offers the
// places people were most likely looking for. It sets its own title, since a missing page
// under /docs is drawn by the docs layout, which gives it none.
const places = [
  { title: 'Get started', desc: 'Install Git Revenge and take the tour.', to: '/docs/guide/install', accent: 'rose' },
  { title: 'Walkthroughs', desc: 'Commit, rebase, resolve and undo, step by step.', to: '/docs/walkthroughs', accent: 'navy' },
  { title: 'Settings and shortcuts', desc: 'Every setting, and every key.', to: '/docs/guide/settings', accent: 'amber' },
];

export default function NotFoundContent() {
  const logo = useBaseUrl('/img/logo.svg');
  return (
    <main>
      <PageMetadata title="Page not found" />
      <section className="hero-banner">
        <img src={logo} alt="" width="88" height="88" style={{ marginBottom: '1.25rem' }} />
        <div className="hero-eyebrow">page not found</div>
        <h1 className="hero-title">
          Nothing at this ref.<br />
          <em className="hero-title-em">Maybe it was rebased.</em>
        </h1>
        <p className="hero-subtitle">
          There is no page at this address. It may have moved when the docs were reorganised;
          one of these is probably what you were after.
        </p>
      </section>
      <section className="doc-links-section">
        <div className="doc-links-grid doc-links-grid--three">
          {places.map(({ title, desc, to, accent }) => (
            <Link key={title} className={`doc-card doc-card--${accent}`} to={to}>
              <div className="doc-card-title">{title}</div>
              <div className="doc-card-desc">{desc}</div>
            </Link>
          ))}
        </div>
        <p className="not-found-report">
          Followed a link here from these docs?{' '}
          <Link to="https://github.com/Git-Revenge/GitRevenge/issues/new?title=Broken+link+on+the+docs+site">
            Say which one
          </Link>
          , and it will be fixed.
        </p>
      </section>
    </main>
  );
}
