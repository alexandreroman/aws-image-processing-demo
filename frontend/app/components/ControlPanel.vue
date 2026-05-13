<script setup lang="ts">
import type { SessionSummary } from '~/composables/useApi';

const props = defineProps<{
  // When provided, the panel shows live counts instead of just the
  // explainer block. Used by /sessions/[id].vue.
  summary?: SessionSummary;
  // Optional Temporal UI link override (defaults to runtime config).
  temporalUiUrl?: string;
  // When true, hides the slider + start button (read-only mode).
  readOnly?: boolean;
}>();

const config = useRuntimeConfig();
const api = useApi();
const toast = useToast();

const SAMPLE_COUNT = 20;
const SAMPLE_BUCKET = 'temporal-aws-autoscaling-demo-images-local';

const count = ref(20);
const submitting = ref(false);

function pickRandomSamples(n: number): { bucket: string; key: string }[] {
  // Sample 1..SAMPLE_COUNT, with replacement allowed when n > SAMPLE_COUNT.
  const out: { bucket: string; key: string }[] = [];
  const pool = Array.from({ length: SAMPLE_COUNT }, (_, i) => i + 1);
  for (let i = 0; i < n; i++) {
    const idx = Math.floor(Math.random() * pool.length);
    const sampleId = pool[idx]!;
    out.push({ bucket: SAMPLE_BUCKET, key: `samples/${sampleId}.jpg` });
  }
  return out;
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

const temporalLink = computed(
  () => props.temporalUiUrl ?? config.public.temporalUiUrl,
);
</script>

<template>
  <section class="card p-5 space-y-5">
    <header>
      <h2 class="text-sm font-semibold text-ink-500 uppercase tracking-wide">
        Control panel
      </h2>
    </header>

    <div v-if="!readOnly" class="space-y-3">
      <label class="block">
        <span class="text-sm font-medium text-ink-700">
          Images in burst: <span class="font-mono text-primary">{{ count }}</span>
        </span>
        <input
          v-model.number="count"
          type="range"
          min="1"
          max="50"
          step="1"
          class="mt-2 w-full accent-primary"
        >
        <div class="flex justify-between text-xs text-ink-400 mt-1">
          <span>1</span>
          <span>50</span>
        </div>
      </label>

      <button
        type="button"
        class="btn-primary-lg w-full"
        :disabled="submitting"
        @click="startBurst"
      >
        <span v-if="submitting">Starting…</span>
        <span v-else>Start burst</span>
      </button>
    </div>

    <div v-if="summary" class="space-y-2 border-t border-ink-100 pt-4">
      <div class="flex justify-between text-sm">
        <span class="text-ink-500">Total</span>
        <span class="font-mono font-semibold">{{ summary.total }}</span>
      </div>
      <div class="flex justify-between text-sm">
        <span class="text-ink-500">Running</span>
        <span class="font-mono font-semibold text-primary">
          {{ summary.running }}
        </span>
      </div>
      <div class="flex justify-between text-sm">
        <span class="text-ink-500">Completed</span>
        <span class="font-mono font-semibold text-emerald-600">
          {{ summary.completed }}
        </span>
      </div>
      <div class="flex justify-between text-sm">
        <span class="text-ink-500">Failed</span>
        <span class="font-mono font-semibold text-rose-600">
          {{ summary.failed }}
        </span>
      </div>
    </div>

    <a
      :href="temporalLink"
      target="_blank"
      rel="noopener noreferrer"
      class="btn-ghost w-full justify-center border border-primary-100"
    >
      Open in Temporal UI
      <span aria-hidden="true">&rarr;</span>
    </a>
  </section>
</template>
