// Poll a single pipeline with adaptive backoff: start at 1s, slow to 2s after
// 10s, then 3s after 30s. Stops once everything is done (running === 0 &&
// total > 0) or when the component using it unmounts.

import { useIntervalFn } from '@vueuse/core';
import type { Pipeline, PipelineSummary, WorkflowItem } from './useApi';

const POLL_FAST_MS = 1_000;
const POLL_MEDIUM_MS = 2_000;
const POLL_SLOW_MS = 3_000;
const SLOW_AFTER_MS = 30_000;
const MEDIUM_AFTER_MS = 10_000;

export interface UsePipelineReturn {
  pipeline: Ref<Pipeline | null>;
  summary: ComputedRef<PipelineSummary>;
  workflows: ComputedRef<WorkflowItem[]>;
  loading: Ref<boolean>;
  error: Ref<Error | null>;
  isPolling: Ref<boolean>;
  refresh: () => Promise<void>;
  stop: () => void;
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
  const loading = ref(false);
  const error = ref<Error | null>(null);

  const summary = computed<PipelineSummary>(
    () => pipeline.value?.summary ?? EMPTY_SUMMARY,
  );
  const workflows = computed<WorkflowItem[]>(
    () => pipeline.value?.workflows ?? [],
  );

  async function refresh() {
    const id = toValue(pipelineId);
    if (!id) {
      return;
    }
    loading.value = true;
    try {
      pipeline.value = await api.getPipeline(id);
      error.value = null;
    } catch (err) {
      error.value = err instanceof Error ? err : new Error(String(err));
    } finally {
      loading.value = false;
    }
  }

  const pollIntervalMs = ref(POLL_FAST_MS);
  let pollStartedAt = 0;

  function updatePollInterval() {
    const elapsed = Date.now() - pollStartedAt;
    const next
      = elapsed >= SLOW_AFTER_MS
        ? POLL_SLOW_MS
        : elapsed >= MEDIUM_AFTER_MS
          ? POLL_MEDIUM_MS
          : POLL_FAST_MS;
    if (next !== pollIntervalMs.value) {
      pollIntervalMs.value = next;
    }
  }

  const { pause, resume, isActive } = useIntervalFn(
    () => {
      updatePollInterval();
      void refresh();
    },
    pollIntervalMs,
    { immediate: false, immediateCallback: false },
  );

  function startPolling() {
    pollStartedAt = Date.now();
    pollIntervalMs.value = POLL_FAST_MS;
    resume();
  }

  // Stop polling automatically when everything is done.
  watch(summary, (s) => {
    if (s.total > 0 && s.running === 0) {
      pause();
    }
  });

  watch(
    () => toValue(pipelineId),
    (id) => {
      if (!id) {
        pause();
        return;
      }
      void refresh();
      startPolling();
    },
    { immediate: true },
  );

  onUnmounted(() => {
    pause();
  });

  return {
    pipeline,
    summary,
    workflows,
    loading,
    error,
    isPolling: isActive,
    refresh,
    stop: pause,
  };
}
