// @ts-check
const { execSync } = require('child_process');
const risInk = require('./src/prism/ris-ink');
const repoLinks = require('./src/remark/repo-links');

// The version shown in the navbar is the newest stable release tag, never a release
// candidate. A checkout without tags (a shallow clone) shows "dev".
function latestStableTag() {
  try {
    const tags = execSync("git -c versionsort.suffix=- tag --list 'v[0-9]*' --sort=-v:refname", {
      stdio: ['ignore', 'pipe', 'ignore'],
    })
      .toString()
      .split('\n')
      .filter((tag) => /^v\d+\.\d+\.\d+$/.test(tag));
    return tags[0] || 'dev';
  } catch {
    return 'dev';
  }
}
const version = latestStableTag();

// The site is served at the root of gitrevenge.com (GitHub Pages, with static/CNAME naming
// the domain), so every build uses /. BASE_URL overrides it.
const baseUrl = process.env.BASE_URL || '/';

/** @type {import('@docusaurus/types').Config} */
const config = {
  title: 'Git Revenge',
  tagline: 'A fast, local-only desktop git client for Windows, macOS and Linux, built for very large repositories.',
  favicon: 'img/favicon.svg',

  url: 'https://gitrevenge.com',
  baseUrl,

  organizationName: 'Git-Revenge',
  projectName: 'GitRevenge',
  trailingSlash: false,

  onBrokenLinks: 'throw',
  onBrokenAnchors: 'throw',
  markdown: {
    // Read .md files as CommonMark, not MDX: the docs are written for GitHub, and their
    // prose holds <kbd>, {…} and the like.
    format: 'detect',
    hooks: {
      onBrokenMarkdownLinks: 'throw',
    },
  },

  headTags: [
    // What a phone shows for the site saved to its home screen.
    {
      tagName: 'link',
      attributes: { rel: 'apple-touch-icon', href: `${baseUrl}img/apple-touch-icon.png` },
    },
  ],

  i18n: {
    defaultLocale: 'en',
    locales: ['en'],
  },

  plugins: [require.resolve('./src/plugins/sidebar-complete.js')],

  presets: [
    [
      'classic',
      /** @type {import('@docusaurus/preset-classic').Options} */
      ({
        docs: {
          path: '../docs',
          sidebarPath: require.resolve('./sidebars.js'),
          editUrl: ({ docPath }) => `https://github.com/Git-Revenge/GitRevenge/edit/main/docs/${docPath}`,
          beforeDefaultRemarkPlugins: [repoLinks],
        },
        blog: false,
        theme: {
          customCss: require.resolve('./src/css/custom.css'),
        },
      }),
    ],
  ],

  themeConfig:
    /** @type {import('@docusaurus/preset-classic').ThemeConfig} */
    ({
      image: 'img/social-card.png',
      colorMode: {
        defaultMode: 'light',
        disableSwitch: false,
        respectPrefersColorScheme: true,
      },
      navbar: {
        title: 'Git Revenge',
        logo: {
          alt: 'Git Revenge',
          src: 'img/logo.svg',
        },
        items: [
          {
            type: 'docSidebar',
            sidebarId: 'mainSidebar',
            position: 'left',
            label: 'Docs',
          },
          {
            href: 'https://github.com/Git-Revenge/GitRevenge/releases',
            label: 'Releases',
            position: 'left',
          },
          {
            type: 'html',
            position: 'right',
            value: `<span class="navbar-version-badge">${version}</span>`,
          },
          {
            href: 'https://github.com/sponsors/risadams',
            label: 'Sponsor',
            position: 'right',
            className: 'header-sponsor-link',
          },
          {
            href: 'https://github.com/Git-Revenge/GitRevenge',
            position: 'right',
            className: 'header-github-link',
            'aria-label': 'GitHub repository',
          },
        ],
      },
      footer: {
        style: 'dark',
        links: [],
        copyright: `
          <div class="footer-bar">
            <div class="footer-bar__brand">
              <svg viewBox="0 0 64 64" width="20" height="20" aria-hidden="true">
                <path d="M11 8 H18 V58 H11 Z" fill="#5ba3b8"></path>
                <path d="M18 11 C28 6 39 16 56 10 V39 C39 45 28 35 18 41 Z" fill="#e24a8a"></path>
              </svg>
              <span><em>Git Revenge</em>, a git client</span>
            </div>
            <nav class="footer-bar__links" aria-label="Footer navigation">
              <a href="https://github.com/Git-Revenge/GitRevenge">GitHub</a>
              <a href="https://github.com/Git-Revenge/GitRevenge/releases">Releases</a>
              <a href="https://github.com/Git-Revenge/GitRevenge/issues">Issues</a>
              <a href="https://github.com/Git-Revenge/GitRevenge/security/policy">Security</a>
              <a href="${baseUrl}docs/privacy">Privacy</a>
              <a href="https://github.com/sponsors/risadams">Sponsor</a>
            </nav>
            <div class="footer-bar__copy">© ${new Date().getFullYear()} Ris Adams · Apache-2.0 · ${version}</div>
          </div>
        `,
      },
      prism: {
        theme: risInk.light,
        darkTheme: risInk.dark,
        additionalLanguages: ['bash', 'json', 'go', 'powershell', 'ini', 'diff'],
      },
    }),
};

module.exports = config;
