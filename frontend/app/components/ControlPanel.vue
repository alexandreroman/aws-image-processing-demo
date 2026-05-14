<script setup lang="ts">
import type { SessionSummary } from '~/composables/useApi';

const props = defineProps<{
  summary?: SessionSummary;
  readOnly?: boolean;
}>();

const api = useApi();
const toast = useToast();

const SAMPLE_COUNT = 50;
const SAMPLE_BUCKET = 'aws-image-processing-demo-images-local';

const count = ref(20);
const submitting = ref(false);

function pickRandomSamples(n: number): { bucket: string; key: string }[] {
  const pool = Array.from({ length: SAMPLE_COUNT }, (_, i) => i + 1);
  const k = Math.min(n, pool.length);
  for (let i = 0; i < k; i++) {
    const j = i + Math.floor(Math.random() * (pool.length - i));
    [pool[i], pool[j]] = [pool[j]!, pool[i]!];
  }
  return pool.slice(0, k).map((id) => ({ bucket: SAMPLE_BUCKET, key: `samples/${id}.jpg` }));
}

async function startBurst() {
  if (submitting.value) return;
  submitting.value = true;
  try {
    const images = pickRandomSamples(count.value);
    const res = await api.startWorkflows(images);
    toast.success(
      'Burst started',
      `Session ${res.sessionId} — ${res.workflowIds.length} workflows`,
    );
    await navigateTo(`/sessions/${res.sessionId}`);
  } catch (err) {
    const message = err instanceof Error ? err.message : String(err);
    toast.error('Failed to start burst', message);
  } finally {
    submitting.value = false;
  }
}

const summaryRows = computed(() => {
  if (!props.summary) return [];
  return [
    { label: 'Total', value: props.summary.total, color: 'text-ink-100' },
    { label: 'Running', value: props.summary.running, color: 'text-primary' },
    { label: 'Completed', value: props.summary.completed, color: 'text-emerald-400' },
    { label: 'Failed', value: props.summary.failed, color: 'text-rose-400' },
  ];
});
</script>

<template>
  <section class="card p-4 space-y-4">
    <header class="flex items-center justify-between">
      <h2 class="stat-label">Control panel</h2>
      <span
        v-if="!readOnly"
        class="chip-primary"
      >
        Live
      </span>
    </header>

    <div v-if="!readOnly" class="space-y-3">
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

      <button
        type="button"
        class="btn-primary-lg w-full"
        :disabled="submitting"
        @click="startBurst"
      >
        <span v-if="submitting">Starting…</span>
        <span v-else>Start burst →</span>
      </button>
    </div>

    <dl
      v-if="summary"
      class="grid grid-cols-2 gap-1.5 border-t border-surface-border pt-3"
    >
      <div
        v-for="row in summaryRows"
        :key="row.label"
        class="flex items-center justify-between rounded-md bg-surface-hover/60
          px-2.5 py-1.5"
      >
        <span class="text-[11px] text-ink-300">{{ row.label }}</span>
        <span
          :class="['font-mono font-semibold tabular-nums text-sm', row.color]"
        >
          {{ row.value }}
        </span>
      </div>
    </dl>
  </section>
</template>
