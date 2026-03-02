import { defineConfig } from 'astro/config';
import starlight from '@astrojs/starlight';

export default defineConfig({
  site: 'https://kinder.patrykgolabek.dev',
  integrations: [
    starlight({
      title: 'kinder',
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
