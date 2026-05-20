<script setup lang="ts">
definePageMeta({
  // SPA fallback: rendered client-side only so the dynamic [id] can be
  // read at runtime. See `routeRules` in nuxt.config.ts.
  ssr: false,
});

const route = useRoute();

const pipelineId = computed(() => String(route.params.id ?? ''));

const { summary, workflows, durationMs, loaded, error, refresh } = usePipeline(pipelineId);

function formatSeconds(ms: number | null): string {
  return ms === null ? '…' : `${(ms / 1000).toFixed(1)}s`;
}

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
  <div class="max-w-7xl mx-auto px-4 sm:px-6 lg:px-8 py-6">
    <div class="grid lg:grid-cols-12 gap-4">
      <div class="lg:col-span-8 space-y-4">
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

        <Gallery :workflows="workflows" :expected-count="expectedCount" />
      </div>

      <aside
        class="lg:col-span-4 space-y-4 lg:fixed lg:top-[4.5rem] lg:right-[max(2rem,calc((100vw-80rem)/2+2rem))] lg:w-[calc((min(80rem,100vw)-15rem)/3+3rem)] lg:max-h-[calc(100vh-6rem)] lg:overflow-y-auto lg:z-20 lg:[scrollbar-gutter:stable]"
      >
        <div class="grid grid-cols-2 gap-4">
          <article class="card p-4 animate-fade-in">
            <h2
              class="stat-label"
              title="Distinct execution environments that picked up work for this burst (~1 for ECS, N for Lambda warm sandboxes). Not the count of Lambda invocations."
            >
              Worker instances
            </h2>
            <p
              class="mt-1 font-mono font-semibold tabular-nums text-3xl text-ink-100"
              :aria-busy="workerCount === null ? 'true' : 'false'"
            >
              <template v-if="workerCount === null">…</template>
              <template v-else>{{ workerCount }}</template>
            </p>
          </article>
          <article class="card p-4 animate-fade-in min-h-[5.5rem] relative">
            <Transition name="duration-fade" mode="out-in">
              <div v-if="durationMs !== null" key="done">
                <h2
                  class="stat-label"
                  title="Wall-clock time from the earliest ProcessImage start to the latest completion."
                >
                  Duration
                </h2>
                <p class="mt-1 font-mono font-semibold tabular-nums text-3xl text-ink-100 leading-9">
                  {{ formatSeconds(durationMs) }}
                </p>
              </div>
              <div
                v-else-if="loaded"
                key="running"
                class="absolute inset-0 flex items-center justify-center gap-2 pr-5"
                role="status"
                aria-label="Pipeline running, duration pending"
              >
                <span class="inline-block h-3 w-3 rounded-full bg-emerald-400 animate-pulse" />
                <span class="font-mono font-semibold text-base text-ink-300">Running</span>
              </div>
              <div v-else key="loading">
                <h2 class="stat-label invisible">Duration</h2>
                <p
                  class="mt-1 font-mono font-semibold tabular-nums text-3xl text-ink-100 leading-9"
                  aria-busy="true"
                >
                  …
                </p>
              </div>
            </Transition>
          </article>
        </div>
        <PipelineCharts :workflows="workflows" :summary="summary" />
      </aside>
    </div>
  </div>
</template>

<style scoped>
.duration-fade-enter-active,
.duration-fade-leave-active {
  transition: opacity 300ms ease;
}
.duration-fade-enter-from,
.duration-fade-leave-to {
  opacity: 0;
}
</style>
