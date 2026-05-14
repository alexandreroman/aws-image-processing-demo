<script setup lang="ts">
definePageMeta({
  // SPA fallback: rendered client-side only so the dynamic [id] can be
  // read at runtime. See `routeRules` in nuxt.config.ts.
  ssr: false,
});

const route = useRoute();

const sessionId = computed(() => String(route.params.id ?? ''));

const { summary, workflows, error, refresh } = useSession(sessionId);

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
      </aside>

      <section class="lg:col-span-5">
        <SessionCharts :workflows="workflows" :summary="summary" />
      </section>

      <section class="lg:col-span-4">
        <Gallery :workflows="workflows" />
      </section>
    </div>
  </div>
</template>
