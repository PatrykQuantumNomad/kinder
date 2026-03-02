import { defineConfig } from 'astro/config';
import starlight from '@astrojs/starlight';

export default defineConfig({
  site: 'https://kinder.patrykgolabek.dev',
  integrations: [
    starlight({
      title: 'kinder',
      favicon: '/favicon.svg',
      disable404Route: true,
      head: [
        { tag: 'meta', attrs: { property: 'og:image', content: 'https://kinder.patrykgolabek.dev/og.png' } },
        { tag: 'meta', attrs: { property: 'og:title', content: 'kinder' } },
        { tag: 'meta', attrs: { property: 'og:description', content: 'kind, but with everything you actually need.' } },
        { tag: 'meta', attrs: { name: 'twitter:card', content: 'summary_large_image' } },
        { tag: 'meta', attrs: { name: 'twitter:image', content: 'https://kinder.patrykgolabek.dev/og.png' } },
      ],
      sidebar: [
        { slug: 'installation' },
        { slug: 'quick-start' },
        { slug: 'configuration' },
        {
          label: 'Addons',
          items: [
            { slug: 'addons/metallb' },
            { slug: 'addons/envoy-gateway' },
            { slug: 'addons/metrics-server' },
            { slug: 'addons/coredns' },
            { slug: 'addons/headlamp' },
          ],
        },
      ],
      social: [
        { icon: 'github', label: 'GitHub', href: 'https://github.com/patrykattc/kinder' },
      ],
      customCss: [
        './src/styles/theme.css',
      ],
      components: {
        ThemeSelect: './src/components/ThemeSelect.astro',
      },
    }),
  ],
});
