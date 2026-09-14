// Poll a single pipeline with adaptive backoff: 1s for the first 10s, then
// 2s thereafter. Stops once everything is done (running === 0 && total > 0)
// or when the component using it unmounts.

import { useIntervalFn } from '@vueuse/core';
import type { Pipeline, PipelineSummary, WorkflowItem } from './useApi';

const POLL_FAST_MS = 1_000;
const POLL_SLOW_MS = 2_000;
const SLOW_AFTER_MS = 10_000;

export interface UsePipelineReturn {
  summary: ComputedRef<PipelineSummary>;
  workflows: ComputedRef<WorkflowItem[]>;
  durationMs: ComputedRef<number | null>;
  isDone: ComputedRef<boolean>;
  loaded: ComputedRef<boolean>;
  error: Ref<Error | null>;
  refresh: () => Promise<void>;
}

const EMPTY_SUMMARY: PipelineSummary = {
  total: 0,
  running: 0,
  completed: 0,
  failed: 0,
};

export function usePipeline(pipelineId: MaybeRefOrGetter<string>): UsePipelineReturn {
  const api = useApi();

  const pipeline = ref<Pipeline | null>(null);
  const error = ref<Error | null>(null);

  const summary = computed<PipelineSummary>(
    () => pipeline.value?.summary ?? EMPTY_SUMMARY,
  );
  const workflows = computed<WorkflowItem[]>(
    () => pipeline.value?.workflows ?? [],
  );
  const isDone = computed<boolean>(() => {
    const p = pipeline.value;
    if (!p) return false;
    return p.summary.total > 0 && p.summary.running === 0;
  });
  const durationMs = computed<number | null>(() => (isDone.value ? pipeline.value?.durationMs ?? null : null));
  const loaded = computed<boolean>(() => pipeline.value !== null);

  // Why: backend latency varies wildly under load, so out-of-order responses can
  // overwrite fresh state with stale snapshots and wedge the page after the
  // running-zero watch has paused polling. Track a monotonic sequence and drop
  // responses older than the last one we already applied.
  let nextSeq = 0;
  let lastAppliedSeq = 0;

  async function refresh() {
    const id = toValue(pipelineId);
    if (!id) {
      return;
    }
    const seq = ++nextSeq;
    try {
      const result = await api.getPipeline(id);
      // The page component is reused across /pipelines/A -> /pipelines/B, so a
      // reply for the pipeline we just left must not land on the new one.
      if (id !== toValue(pipelineId)) {
        return;
      }
      if (seq <= lastAppliedSeq) {
        return;
      }
      lastAppliedSeq = seq;
      pipeline.value = result;
      error.value = null;
    } catch (err) {
      if (id !== toValue(pipelineId)) {
        return;
      }
      if (seq <= lastAppliedSeq) {
        return;
      }
      lastAppliedSeq = seq;
      error.value = err instanceof Error ? err : new Error(String(err));
    }
  }

  const pollIntervalMs = ref(POLL_FAST_MS);
  let pollStartedAt = 0;

  function updatePollInterval() {
    const next = Date.now() - pollStartedAt >= SLOW_AFTER_MS
      ? POLL_SLOW_MS
      : POLL_FAST_MS;
    if (next !== pollIntervalMs.value) {
      pollIntervalMs.value = next;
    }
  }

  const { pause, resume } = useIntervalFn(
    () => {
      updatePollInterval();
      void refresh();
    },
    pollIntervalMs,
    { immediate: false, immediateCallback: false },
  );

  watch(isDone, (done) => {
    if (done) {
      pause();
    }
  });

  watch(
    () => toValue(pipelineId),
    (id) => {
      // Drop the previous pipeline's snapshot so the page never renders it
      // under the new id while the first poll is still in flight.
      pipeline.value = null;
      error.value = null;
      if (!id) {
        pause();
        return;
      }
      void refresh();
      pollStartedAt = Date.now();
      pollIntervalMs.value = POLL_FAST_MS;
      resume();
    },
    { immediate: true },
  );

  onUnmounted(() => {
    pause();
  });

  return {
    summary,
    workflows,
    durationMs,
    isDone,
    loaded,
    error,
    refresh,
  };
}
