<script setup lang="ts">
definePageMeta({
  // SPA fallback: rendered client-side only so the dynamic [id] can be
  // read at runtime. See `routeRules` in nuxt.config.ts.
  ssr: false,
});

const route = useRoute();

const sessionId = computed(() => String(route.params.id ?? ''));

const { summary, workflows, error, isPolling, refresh } = useSession(
  sessionId,
);

useHead(() => ({
  title: `Session ${sessionId.value} — AWS Image Processing Demo`,
}));
</script>

<template>
  <div class="max-w-7xl mx-auto px-4 sm:px-6 lg:px-8 py-4 space-y-4">
    <div
      v-if="error"
      class="card border-rose-500/40 bg-rose-500/10 text-rose-200
        px-4 py-3 text-sm"
    >
      <strong class="font-semibold text-rose-100">
        Couldn't load session:
      </strong>
      {{ error.message }}
      <button
        type="button"
        class="ml-2 underline hover:text-rose-100"
        @click="() => refresh()"
      >
        Retry
      </button>
    </div>

    <div class="grid lg:grid-cols-12 gap-4">
      <aside class="lg:col-span-3 space-y-3">
        <ControlPanel :summary="summary" read-only />
        <div
          class="flex items-center gap-2 text-[11px] text-ink-400 px-1"
        >
          <span
            v-if="isPolling"
            class="inline-flex items-center gap-1.5"
          >
            <span
              class="h-1.5 w-1.5 rounded-full bg-primary animate-pulse-glow"
              aria-hidden="true"
            />
            Polling every 1s…
          </span>
          <span v-else class="inline-flex items-center gap-1.5">
            <span
              class="h-1.5 w-1.5 rounded-full bg-emerald-400"
              aria-hidden="true"
            />
            Polling stopped — all done.
          </span>
        </div>
      </aside>

      <section class="lg:col-span-5">
        <MetricsCards :summary="summary" />
      </section>

      <section class="lg:col-span-4">
        <Gallery :workflows="workflows" />
      </section>
    </div>
  </div>
</template>
