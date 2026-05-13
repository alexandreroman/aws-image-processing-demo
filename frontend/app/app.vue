<script setup lang="ts">
const config = useRuntimeConfig();
</script>

<template>
  <div class="min-h-screen flex flex-col">
    <header
      class="sticky top-0 z-30 bg-white/85 backdrop-blur border-b
        border-ink-100"
    >
      <div
        class="max-w-7xl mx-auto px-4 sm:px-6 lg:px-8 h-14 flex items-center
          justify-between"
      >
        <NuxtLink to="/" class="flex items-center gap-2.5 group">
          <span
            class="inline-flex items-center justify-center h-7 w-7 rounded
              bg-primary text-white font-bold text-sm tracking-tight
              group-hover:bg-primary-600 transition-colors"
            aria-hidden="true"
          >
            T
          </span>
          <span class="font-semibold text-ink-800 text-sm sm:text-base">
            Temporal AWS Demo
            <span class="text-ink-400 font-normal hidden sm:inline">
              — Image Processing Burst
            </span>
          </span>
        </NuxtLink>

        <a
          :href="config.public.githubUrl"
          target="_blank"
          rel="noopener noreferrer"
          class="text-sm text-ink-500 hover:text-primary transition-colors"
        >
          GitHub
        </a>
      </div>
    </header>

    <main class="flex-1">
      <NuxtPage />
    </main>

    <ClientOnly>
      <EventToaster />
    </ClientOnly>
  </div>
</template>
