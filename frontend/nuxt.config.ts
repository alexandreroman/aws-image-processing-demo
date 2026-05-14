// https://nuxt.com/docs/api/configuration/nuxt-config
export default defineNuxtConfig({
  compatibilityDate: '2025-01-01',
  devtools: { enabled: true },

  modules: [
    '@nuxtjs/tailwindcss',
    '@vueuse/nuxt',
    '@nuxt/eslint',
  ],

  css: ['~/assets/css/main.css'],

  app: {
    head: {
      title: 'AWS Image Processing Demo — Image Processing Burst',
      meta: [
        { charset: 'utf-8' },
        { name: 'viewport', content: 'width=device-width, initial-scale=1' },
        {
          name: 'description',
          content:
            'Image-processing burst pipeline demonstrating Temporal Cloud + AWS.',
        },
      ],
    },
  },

  runtimeConfig: {
    public: {
      apiBase: process.env.NUXT_PUBLIC_API_BASE ?? 'http://localhost:8000',
      s3PublicUrl:
        process.env.NUXT_PUBLIC_S3_PUBLIC_URL ?? 'http://localhost:4566',
      githubUrl:
        process.env.NUXT_PUBLIC_GITHUB_URL ??
        'https://github.com/alexandreroman/aws-image-processing-demo',
      samplesBucket:
        process.env.NUXT_PUBLIC_SAMPLES_BUCKET ??
        'aws-image-processing-demo-images-local',
    },
  },

  // SSG configuration.
  // - Only `/` is prerendered at build time (full static page).
  // - `/sessions/**` is served as a client-rendered SPA fallback so the
  //   dynamic sessionId can be read from `useRoute()` and the API can be
  //   polled client-side. Combined with CloudFront's "404 -> /index.html"
  //   custom error response, any /sessions/{id} URL resolves correctly.
  nitro: {
    prerender: { routes: ['/'], crawlLinks: false },
    routeRules: { '/sessions/**': { ssr: false } },
  },

  typescript: {
    strict: true,
    typeCheck: false,
  },

  eslint: {
    config: {
      stylistic: false,
    },
  },
});
