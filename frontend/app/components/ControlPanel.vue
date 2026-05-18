<script setup lang="ts">
import type { RuntimeName } from '~/composables/useApi';

const api = useApi();
const toast = useToast();
const config = useRuntimeConfig();

const SAMPLE_COUNT = 50;
const samplesBucket = config.public.samplesBucket;

// Canonical list of runtimes the UI knows how to render. The actual set
// shown is the intersection of this list and what the backend advertises
// via `/api/runtimes`.
const KNOWN_RUNTIMES: readonly RuntimeName[] = ['ecs', 'lambda'] as const;

const RUNTIME_LABELS: Record<RuntimeName, string> = {
  ecs: 'ECS Fargate',
  lambda: 'AWS Lambda',
};

function runtimeLabel(r: RuntimeName): string {
  return RUNTIME_LABELS[r];
}

const count = ref(20);
const submitting = ref(false);
// Empty by default: the selector only appears when the backend advertises
// 2+ runtimes (real AWS deploy). In local dev, /api/runtimes returns []
// and this stays empty, so no fieldset is rendered.
const availableRuntimes = ref<RuntimeName[]>([]);
const selectedRuntime = ref<RuntimeName>('ecs');

const selectedIndex = computed(() => {
  const i = availableRuntimes.value.indexOf(selectedRuntime.value);
  return i < 0 ? 0 : i;
});

onMounted(async () => {
  try {
    const runtimes = await api.getRuntimes();
    const filtered = KNOWN_RUNTIMES.filter((r) =>
      runtimes.some((entry) => entry.name === r),
    );
    availableRuntimes.value = filtered;
    if (filtered.length > 0 && !filtered.includes(selectedRuntime.value)) {
      selectedRuntime.value = filtered[0]!;
    }
  } catch (err) {
    // Initial-load failure shouldn't toast — leave the selector hidden and
    // let the user retry by submitting a burst.
    console.warn('Failed to load runtimes', err);
  }
});

function pickRandomSamples<T>(pool: T[], k: number): T[] {
  const a = [...pool];
  for (let i = a.length - 1; i > 0; i--) {
    const j = Math.floor(Math.random() * (i + 1));
    [a[i], a[j]] = [a[j]!, a[i]!];
  }
  return a.slice(0, k);
}

function pickRandomSampleRefs(n: number): { bucket: string; key: string }[] {
  const pool = Array.from({ length: SAMPLE_COUNT }, (_, i) => i + 1);
  const k = Math.min(n, pool.length);
  return pickRandomSamples(pool, k).map((id) => ({
    bucket: samplesBucket,
    key: `samples/${id}.jpg`,
  }));
}

async function startBurst() {
  if (submitting.value) return;
  submitting.value = true;
  try {
    const images = pickRandomSampleRefs(count.value);
    // Only forward the runtime when the selector is actually rendered —
    // otherwise the backend treats the field as unset and routes via its
    // DefaultTaskQueue.
    const runtime = availableRuntimes.value.length > 1
      ? selectedRuntime.value
      : undefined;
    const res = await api.startWorkflows(images, runtime);
    const summary = res.runtime !== undefined
      ? `Pipeline ${res.pipelineId} — ${res.workflowIds.length} workflows on ${runtimeLabel(res.runtime)}`
      : `Pipeline ${res.pipelineId} — ${res.workflowIds.length} workflows`;
    toast.success('Burst started', summary);
    // Seed the expected slot count so the gallery reserves space before the first poll lands.
    useState<number | null>(`pipeline:expectedCount:${res.pipelineId}`, () => null).value
      = res.workflowIds.length;
    await navigateTo(`/pipelines/${res.pipelineId}`);
  } catch (err) {
    const message = err instanceof Error ? err.message : String(err);
    toast.error('Failed to start burst', message);
  } finally {
    submitting.value = false;
  }
}
</script>

<template>
  <section class="card p-4 flex flex-col gap-4 lg:h-full">
    <header class="flex items-center">
      <h2 class="stat-label">Control panel</h2>
    </header>

    <label class="block">
      <span
        class="flex items-baseline justify-between text-xs font-medium
          text-ink-200"
      >
        <span>Images in burst</span>
        <span class="font-mono text-primary text-lg font-bold tabular-nums">
          {{ count }}
        </span>
      </span>
      <input
        v-model.number="count"
        type="range"
        min="1"
        max="48"
        step="1"
        class="mt-2 w-full accent-primary cursor-pointer"
      >
      <div class="flex justify-between text-[10px] text-ink-400 mt-0.5">
        <span>1</span>
        <span>48</span>
      </div>
    </label>

    <fieldset v-if="availableRuntimes.length > 1" class="block">
      <legend class="text-xs font-medium text-ink-200">
        Worker runtime
      </legend>
      <div
        role="radiogroup"
        aria-label="Worker runtime"
        class="relative isolate mt-2 grid gap-1 p-1 rounded-md
          bg-surface-elevated border border-surface-border"
        :style="{ gridTemplateColumns: `repeat(${availableRuntimes.length}, minmax(0, 1fr))` }"
      >
        <span
          aria-hidden="true"
          class="pointer-events-none absolute top-1 bottom-1 left-1 rounded
            bg-primary shadow-glow transition-transform duration-200 ease-out
            motion-reduce:transition-none"
          :style="{
            width: `calc((100% - 0.5rem - ${availableRuntimes.length - 1} * 0.25rem) / ${availableRuntimes.length})`,
            transform: `translateX(calc(${selectedIndex} * (100% + 0.25rem)))`,
          }"
        />
        <button
          v-for="r in availableRuntimes"
          :key="r"
          type="button"
          role="radio"
          :aria-checked="selectedRuntime === r"
          :tabindex="selectedRuntime === r ? 0 : -1"
          class="relative z-10 text-xs font-medium py-1.5 rounded
            transition-colors focus-visible:outline-none focus-visible:ring-2
            focus-visible:ring-primary/60"
          :class="selectedRuntime === r
            ? 'text-bg'
            : 'text-ink-200 hover:text-ink-100'"
          @click="selectedRuntime = r"
        >
          {{ runtimeLabel(r) }}
        </button>
      </div>
    </fieldset>

    <button
      type="button"
      class="btn-primary-lg w-full lg:mt-auto"
      :disabled="submitting"
      @click="startBurst"
    >
      <span v-if="submitting">Starting…</span>
      <span v-else>Start burst →</span>
    </button>
  </section>
</template>
