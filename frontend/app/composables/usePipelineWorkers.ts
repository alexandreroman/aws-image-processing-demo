// Poll a pipeline's distinct worker count every 3s while the pipeline is
// running. When `done` flips true, fire one final refresh (to count any
// tail-end activity starts that landed between the last poll and "done")
// and then pause. The workerCount stays `null` until the first successful
// fetch so callers can render a skeleton.

import { useIntervalFn } from '@vueuse/core';

const POLL_MS = 3_000;

export interface UsePipelineWorkersReturn {
  workerCount: ComputedRef<number | null>;
  error: Ref<Error | null>;
  refresh: () => Promise<void>;
}

export function usePipelineWorkers(
  pipelineId: MaybeRefOrGetter<string>,
  done: MaybeRefOrGetter<boolean>,
): UsePipelineWorkersReturn {
  const api = useApi();

  const count = ref<number | null>(null);
  const error = ref<Error | null>(null);

  const workerCount = computed<number | null>(() => count.value);

  // Drop out-of-order responses. The pipeline page can pause/resume polling
  // and trigger a final refresh; without sequencing, a slow earlier response
  // could clobber a fresher value.
  let nextSeq = 0;
  let lastAppliedSeq = 0;

  async function refresh() {
    const id = toValue(pipelineId);
    if (!id) {
      return;
    }
    const seq = ++nextSeq;
    try {
      const result = await api.getPipelineWorkers(id);
      if (seq <= lastAppliedSeq) {
        return;
      }
      lastAppliedSeq = seq;
      count.value = result.workerCount;
      error.value = null;
    } catch (err) {
      if (seq <= lastAppliedSeq) {
        return;
      }
      lastAppliedSeq = seq;
      error.value = err instanceof Error ? err : new Error(String(err));
    }
  }

  const { pause, resume } = useIntervalFn(
    () => {
      void refresh();
    },
    POLL_MS,
    { immediate: false, immediateCallback: false },
  );

  watch(
    () => toValue(pipelineId),
    (id) => {
      if (!id) {
        pause();
        return;
      }
      count.value = null;
      lastAppliedSeq = 0;
      nextSeq = 0;
      void refresh();
      if (!toValue(done)) {
        resume();
      }
    },
    { immediate: true },
  );

  watch(
    () => toValue(done),
    (isDone) => {
      if (!isDone) {
        return;
      }
      pause();
      // One final read so identities that started between the last poll
      // and the "done" flip are still counted.
      void refresh();
    },
  );

  onUnmounted(() => {
    pause();
  });

  return {
    workerCount,
    error,
    refresh,
  };
}
