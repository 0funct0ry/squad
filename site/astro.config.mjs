// @ts-check
import { defineConfig } from 'astro/config';

import starlight from '@astrojs/starlight';
import tailwindcss from '@tailwindcss/vite';

// https://astro.build/config
export default defineConfig({
  site: 'https://0funct0ry.github.io',
  base: '/squad',
  integrations: [
    starlight({
      title: 'squad',
      description: 'A single-binary, web-based SQLite client.',
      social: [
        { icon: 'github', label: 'GitHub', href: 'https://github.com/0funct0ry/squad' },
      ],
      customCss: ['./src/styles/tokens.css'],
      favicon: '/favicon.svg',
      sidebar: [
        {
          label: 'Getting Started',
          items: [
            { label: 'Installation', slug: 'docs/getting-started/installation' },
            { label: 'Quickstart', slug: 'docs/getting-started/quickstart' },
            { label: 'Concepts', slug: 'docs/getting-started/concepts' },
          ],
        },
        {
          label: 'CLI Reference',
          items: [
            { label: 'Global Flags', slug: 'docs/cli-reference/global-flags' },
            { label: 'Root Command (squad <db>)', slug: 'docs/cli-reference/root-command' },
            { label: 'squad sandbox', slug: 'docs/cli-reference/sandbox' },
            { label: 'squad cli', slug: 'docs/cli-reference/cli' },
            { label: 'Dot-command reference', slug: 'docs/cli-reference/dot-commands' },
            { label: 'Template function reference', slug: 'docs/cli-reference/template-functions' },
            { label: 'squad version', slug: 'docs/cli-reference/version' },
          ],
        },
        {
          label: 'Web UI Guide',
          items: [
            { label: 'Tables & Schema', slug: 'docs/web-ui-guide/tables-and-schema' },
            { label: 'SQL Editor', slug: 'docs/web-ui-guide/sql-editor' },
            { label: 'Export', slug: 'docs/web-ui-guide/export' },
            { label: 'Table Editor', slug: 'docs/web-ui-guide/table-editor' },
            { label: 'Seed Data', slug: 'docs/web-ui-guide/seed-data' },
            { label: 'Seeding Walkthrough', slug: 'docs/web-ui-guide/seed-data-walkthrough' },
            { label: 'Generator Reference', slug: 'docs/web-ui-guide/generator-reference' },
            { label: 'Foreign Keys', slug: 'docs/web-ui-guide/foreign-keys' },
          ],
        },
        {
          label: 'REST API',
          items: [
            { label: 'API Envelope', slug: 'docs/rest-api/api-envelope' },
            { label: 'Auto-REST', slug: 'docs/rest-api/auto-rest' },
          ],
        },
        {
          label: 'Security Model',
          items: [{ label: 'Security Model', slug: 'docs/security-model' }],
        },
        {
          label: 'FAQ',
          items: [{ label: 'FAQ', slug: 'docs/faq' }],
        },
      ],
    }),
  ],

  vite: {
    plugins: [tailwindcss()],
  },
});