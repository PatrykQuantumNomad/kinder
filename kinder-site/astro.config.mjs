import { defineConfig } from 'astro/config';
import starlight from '@astrojs/starlight';

export default defineConfig({
  site: 'https://kinder.patrykgolabek.dev',
  integrations: [
    starlight({
      title: 'kinder',
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
