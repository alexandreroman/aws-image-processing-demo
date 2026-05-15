<script setup lang="ts"></script>

<template>
  <div class="min-h-screen flex flex-col">
    <header
      class="sticky top-0 z-30 bg-bg/80 backdrop-blur-md border-b
        border-surface-border"
    >
      <div
        class="max-w-7xl mx-auto px-4 sm:px-6 lg:px-8 h-12 flex items-center
          justify-between gap-4"
      >
        <NuxtLink to="/" class="flex items-center gap-2.5 group min-w-0">
          <span
            class="inline-flex items-center justify-center h-7 w-7 rounded-md
              bg-gradient-to-br from-primary to-iris text-bg shadow-glow"
            aria-hidden="true"
          >
            <svg
              viewBox="0 0 24 24"
              fill="none"
              stroke="currentColor"
              stroke-width="2.2"
              stroke-linecap="round"
              stroke-linejoin="round"
              class="h-4 w-4"
            >
              <path d="M18 22H4a2 2 0 0 1-2-2V6" />
              <path d="m22 13-1.296-1.296a2.41 2.41 0 0 0-3.408 0L11 18" />
              <circle cx="12" cy="8" r="2" />
              <rect x="6" y="2" width="16" height="16" rx="2" />
            </svg>
          </span>
          <img
            src="/aws-logo.png"
            alt="AWS"
            class="h-5 w-auto shrink-0 ml-1 mb-2 translate-y-[5px]"
            loading="lazy"
            decoding="async"
            draggable="false"
          >
          <span
            class="font-semibold text-ink-100 text-sm sm:text-[15px] truncate"
          >
            Image Processing Demo
          </span>
        </NuxtLink>

        <nav class="flex items-center gap-2">
          <div class="flex items-center gap-2 text-xs text-ink-400 group">
            <span class="hidden sm:inline uppercase tracking-[0.12em] font-medium">powered by</span>
            <img
              src="https://images.ctfassets.net/0uuz8ydxyd9p/2ctnUPEhKA75tYnrl2Kzvj/90563965bc4ea2af9442b6eb4ba43180/Temporal_LogoLockup_Horizontal_light_1_2x.png"
              alt="Temporal"
              class="h-8 w-auto opacity-90 transition-opacity"
              loading="lazy"
            >
          </div>
        </nav>
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
