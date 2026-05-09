import { defineConfig } from 'astro/config';
import starlight from '@astrojs/starlight';

export default defineConfig({
  site: 'https://kinder.patrykgolabek.dev',
  integrations: [
    starlight({
      title: 'kinder',
      favicon: '/favicon.ico',
      disable404Route: true,
      head: [
        // Open Graph
        { tag: 'meta', attrs: { property: 'og:image', content: 'https://kinder.patrykgolabek.dev/og.png' } },
        { tag: 'meta', attrs: { property: 'og:title', content: 'kinder' } },
        { tag: 'meta', attrs: { property: 'og:description', content: 'kind, but with everything you actually need.' } },
        // Twitter Card
        { tag: 'meta', attrs: { name: 'twitter:card', content: 'summary_large_image' } },
        { tag: 'meta', attrs: { name: 'twitter:image', content: 'https://kinder.patrykgolabek.dev/og.png' } },
        { tag: 'meta', attrs: { name: 'twitter:title', content: 'kinder' } },
        { tag: 'meta', attrs: { name: 'twitter:description', content: 'kind, but with everything you actually need.' } },
        // Author & SEO
        { tag: 'meta', attrs: { name: 'author', content: 'Patryk Golabek' } },
        { tag: 'meta', attrs: { name: 'keywords', content: 'kinder, kind, kubernetes, local cluster, docker, metallb, envoy gateway, metrics server, headlamp, k8s, devtools' } },
        { tag: 'link', attrs: { rel: 'author', href: 'https://patrykgolabek.dev' } },
        // JSON-LD Structured Data
        { tag: 'script', attrs: { type: 'application/ld+json' }, content: JSON.stringify({
          '@context': 'https://schema.org',
          '@graph': [
            {
              '@type': 'SoftwareApplication',
              name: 'kinder',
              description: 'kind, but with everything you actually need. A batteries-included tool for running local Kubernetes clusters with MetalLB, Envoy Gateway, Metrics Server, CoreDNS tuning, and Headlamp pre-installed.',
              url: 'https://kinder.patrykgolabek.dev',
              applicationCategory: 'DeveloperApplication',
              operatingSystem: 'Linux, macOS, Windows',
              offers: { '@type': 'Offer', price: '0', priceCurrency: 'USD' },
              author: { '@type': 'Person', name: 'Patryk Golabek', url: 'https://patrykgolabek.dev' },
              codeRepository: 'https://github.com/PatrykQuantumNomad/kinder',
              programmingLanguage: 'Go',
              license: 'https://github.com/PatrykQuantumNomad/kinder/blob/main/LICENSE',
            },
            {
              '@type': 'WebSite',
              name: 'kinder',
              url: 'https://kinder.patrykgolabek.dev',
              description: 'Documentation for kinder — kind, but with everything you actually need.',
              author: { '@type': 'Person', name: 'Patryk Golabek', url: 'https://patrykgolabek.dev' },
            },
          ],
        }) },
        // Dark theme enforcement
        { tag: 'script', content: "localStorage.setItem('starlight-theme','dark');document.documentElement.dataset.theme='dark';" },
      ],
      sidebar: [
        { slug: 'installation' },
        { slug: 'quick-start' },
        { slug: 'configuration' },
        {
          label: 'Addons',
          collapsed: true,
          items: [
            { slug: 'addons/metallb' },
            { slug: 'addons/envoy-gateway' },
            { slug: 'addons/metrics-server' },
            { slug: 'addons/coredns' },
            { slug: 'addons/headlamp' },
            { slug: 'addons/local-registry' },
            { slug: 'addons/cert-manager' },
            { slug: 'addons/local-path-provisioner' },
            { slug: 'addons/nvidia-gpu' },
          ],
        },
        {
          label: 'Guides',
          collapsed: true,
          items: [
            { slug: 'guides/tls-web-app' },
            { slug: 'guides/hpa-auto-scaling' },
            { slug: 'guides/local-dev-workflow' },
            { slug: 'guides/dynamic-storage' },
            { slug: 'guides/host-directory-mounting' },
            { slug: 'guides/multi-version-clusters' },
            { slug: 'guides/k8s-1-36-whats-new' },
            { slug: 'guides/working-offline' },
          ],
        },
        {
          label: 'CLI Reference',
          collapsed: true,
          items: [
            { slug: 'cli-reference/profile-comparison' },
            { slug: 'cli-reference/json-output' },
            { slug: 'cli-reference/troubleshooting' },
            { slug: 'cli-reference/load-images' },
            { slug: 'cli-reference/cluster-lifecycle' },
            { slug: 'cli-reference/snapshot' },
            { slug: 'cli-reference/dev' },
          ],
        },
        { slug: 'known-issues' },
        { slug: 'changelog' },
      ],
      social: [
        { icon: 'github', label: 'GitHub', href: 'https://github.com/PatrykQuantumNomad/kinder' },
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
