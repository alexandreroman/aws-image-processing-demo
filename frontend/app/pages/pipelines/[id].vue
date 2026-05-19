<script setup lang="ts">
definePageMeta({
  // SPA fallback: rendered client-side only so the dynamic [id] can be
  // read at runtime. See `routeRules` in nuxt.config.ts.
  ssr: false,
});

const route = useRoute();

const pipelineId = computed(() => String(route.params.id ?? ''));

const { summary, workflows, error, refresh } = usePipeline(pipelineId);

const done = computed(() => summary.value.total > 0 && summary.value.running === 0);

const { workerCount } = usePipelineWorkers(pipelineId, done);

const navExpectedCount = computed<number>(() => {
  const id = pipelineId.value;
  if (!id) return 0;
  return useState<number | null>(`pipeline:expectedCount:${id}`, () => null).value ?? 0;
});

const expectedCount = computed<number>(() =>
  Math.max(navExpectedCount.value, summary.value.total, workflows.value.length),
);

useHead(() => ({
  title: `Pipeline ${pipelineId.value} — AWS Image Processing Demo`,
}));
</script>

<template>
  <div class="max-w-7xl mx-auto px-4 sm:px-6 lg:px-8 py-6 space-y-4">
    <div
      v-if="error"
      class="card border-rose-500/40 bg-rose-500/10 text-rose-200
        px-4 py-3 text-sm"
    >
      <strong class="font-semibold text-rose-100">
        Couldn't load pipeline:
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
      <section class="lg:col-span-8">
        <Gallery :workflows="workflows" :expected-count="expectedCount" />
      </section>

      <aside class="lg:col-span-4 space-y-4">
        <article class="card p-4 animate-fade-in">
          <h2 class="stat-label">Workers</h2>
          <p
            class="mt-1 font-mono font-semibold tabular-nums text-3xl text-ink-100"
            :aria-busy="workerCount === null ? 'true' : 'false'"
          >
            <template v-if="workerCount === null">…</template>
            <template v-else>{{ workerCount }}</template>
          </p>
        </article>
        <PipelineCharts :workflows="workflows" :summary="summary" />
      </aside>
    </div>
  </div>
</template>
