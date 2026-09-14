<script setup lang="ts">
import type { RuntimeName, S3Ref } from '~/composables/useApi';

const api = useApi();
const toast = useToast();

const SAMPLE_COUNT = 50;

// Canonical list of runtimes the UI knows how to render. The actual set
// shown is the intersection of this list and what the backend advertises
// via `/api/runtimes`.
const KNOWN_RUNTIMES: readonly RuntimeName[] = ['ecs', 'lambda'] as const;

const RUNTIME_LABELS: Record<RuntimeName, string> = {
  ecs: 'ECS Fargate',
  lambda: 'AWS Lambda',
};

const count = ref(20);
const submitting = ref(false);
// Empty by default — populated once /api/runtimes resolves with both
// runtimes (AWS-deployed environments). In local dev the array stays
// empty and the fieldset renders disabled.
const availableRuntimes = ref<RuntimeName[]>([]);
const selectedRuntime = ref<RuntimeName>('ecs');

// The backend advertises either both runtimes or none, so a partial list
// means the runtime registry is not configured.
const awsAvailable = computed(() => availableRuntimes.value.length === KNOWN_RUNTIMES.length);

const selectedIndex = computed(() => KNOWN_RUNTIMES.indexOf(selectedRuntime.value));

onMounted(async () => {
  try {
    const runtimes = await api.getRuntimes();
    availableRuntimes.value = KNOWN_RUNTIMES.filter((r) =>
      runtimes.some((entry) => entry.name === r),
    );
  } catch (err) {
    // Initial-load failure shouldn't toast — leave the selector disabled
    // and let the user retry by submitting a burst.
    console.warn('Failed to load runtimes', err);
  }
});

// Fisher-Yates shuffle over the whole sample pool, then take the first `n`.
function pickRandomSampleKeys(n: number): S3Ref[] {
  const ids = Array.from({ length: SAMPLE_COUNT }, (_, i) => i + 1);
  for (let i = ids.length - 1; i > 0; i--) {
    const j = Math.floor(Math.random() * (i + 1));
    [ids[i], ids[j]] = [ids[j]!, ids[i]!];
  }
  return ids.slice(0, n).map((id) => ({ key: `samples/${id}.jpg` }));
}

async function startBurst() {
  if (submitting.value) return;
  submitting.value = true;
  try {
    const images = pickRandomSampleKeys(count.value);
    // Only forward the runtime when AWS is actually wired — otherwise
    // the backend treats the field as unset and routes via its
    // DefaultTaskQueue.
    const runtime = awsAvailable.value
      ? selectedRuntime.value
      : undefined;
    const res = await api.startWorkflows(images, runtime);
    const summary = res.runtime !== undefined
      ? `Pipeline ${res.pipelineId} — ${res.workflowIds.length} workflows on ${RUNTIME_LABELS[res.runtime]}`
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
        :disabled="submitting"
        class="mt-2 w-full accent-primary cursor-pointer disabled:opacity-50 disabled:cursor-not-allowed"
      >
      <div class="flex justify-between text-[10px] text-ink-400 mt-0.5">
        <span>1</span>
        <span>48</span>
      </div>
    </label>

    <fieldset :disabled="submitting || !awsAvailable" class="block">
      <legend class="text-xs font-medium text-ink-200">
        Worker runtime
      </legend>
      <div
        class="relative isolate mt-2 grid grid-cols-2 gap-1 p-1 rounded-md
          bg-surface-elevated border border-surface-border"
      >
        <!-- Sliding pill behind the labels: one column wide, minus the
             container padding (0.5rem) and the single 0.25rem gap. -->
        <span
          v-if="awsAvailable"
          aria-hidden="true"
          class="pointer-events-none absolute top-1 bottom-1 left-1 rounded-sm
            w-[calc((100%-0.75rem)/2)] bg-primary shadow-glow
            transition-transform duration-200 ease-out
            motion-reduce:transition-none"
          :style="{ transform: `translateX(calc(${selectedIndex} * (100% + 0.25rem)))` }"
        />
        <!-- Native radios (visually hidden) so the browser provides arrow-key
             navigation, focus management and the disabled cascade. -->
        <label
          v-for="r in KNOWN_RUNTIMES"
          :key="r"
          class="relative z-10 text-xs font-medium py-1.5 rounded-sm text-center
            cursor-pointer transition-colors
            has-[:focus-visible]:ring-2 has-[:focus-visible]:ring-primary/60
            has-[:disabled]:opacity-50 has-[:disabled]:cursor-not-allowed"
          :class="awsAvailable && selectedRuntime === r
            ? 'text-bg'
            : 'text-ink-200 hover:text-ink-100'"
        >
          <input
            v-model="selectedRuntime"
            type="radio"
            name="runtime"
            :value="r"
            class="sr-only"
          >
          {{ RUNTIME_LABELS[r] }}
        </label>
      </div>
    </fieldset>

    <button
      type="button"
      class="btn-primary-lg w-full mt-auto"
      :disabled="submitting"
      @click="startBurst"
    >
      <span v-if="submitting">Starting…</span>
      <span v-else>Start burst →</span>
    </button>
  </section>
</template>
