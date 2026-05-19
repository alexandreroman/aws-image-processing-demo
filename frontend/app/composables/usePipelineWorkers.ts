// Poll a pipeline's distinct worker count. While the pipeline is running we
// poll every 3s. Once `done` flips true we slow to 5s and keep polling until
// the worker count has been identical for 3 consecutive reads (or 120s
// elapses), then stop. This catches identities that land seconds after the
// apparent completion on larger Lambda bursts without forcing a manual
// refresh. The workerCount stays `null` until the first successful fetch so
// callers can render a skeleton.

import { useIntervalFn } from '@vueuse/core';

const POLL_MS = 3_000;
const POST_DONE_POLL_MS = 5_000;
const POST_DONE_STABLE_POLLS = 3;
const POST_DONE_MAX_MS = 120_000;

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

  // Drop out-of-order responses: pause/resume + manual refresh can otherwise
  // let a slow earlier reply clobber a fresher value.
  let nextSeq = 0;
  let lastAppliedSeq = 0;

  // Post-done stability tracking.
  let stableCount = 0;
  let lastStableValue: number | null = null;
  let postDoneDeadline = 0;

  const interval = computed(() => (toValue(done) ? POST_DONE_POLL_MS : POLL_MS));

  async function refresh() {
    const id = toValue(pipelineId);
    if (!id) return;
    const seq = ++nextSeq;
    try {
      const result = await api.getPipelineWorkers(id);
      if (seq <= lastAppliedSeq) return;
      lastAppliedSeq = seq;
      count.value = result.workerCount;
      error.value = null;
    } catch (err) {
      if (seq <= lastAppliedSeq) return;
      lastAppliedSeq = seq;
      error.value = err instanceof Error ? err : new Error(String(err));
    }
  }

  const { pause, resume } = useIntervalFn(
    async () => {
      await refresh();
      if (!toValue(done)) return;
      // Post-done stop conditions: stable count or deadline.
      const after = count.value;
      if (after !== null && after === lastStableValue) {
        stableCount += 1;
      } else {
        lastStableValue = after;
        stableCount = after === null ? 0 : 1;
      }
      if (stableCount >= POST_DONE_STABLE_POLLS || Date.now() >= postDoneDeadline) {
        pause();
      }
    },
    interval,
    { immediate: false, immediateCallback: false },
  );

  function resetStability() {
    stableCount = 0;
    lastStableValue = null;
    postDoneDeadline = Date.now() + POST_DONE_MAX_MS;
  }

  watch(
    () => toValue(pipelineId),
    (id) => {
      pause();
      if (!id) return;
      count.value = null;
      lastAppliedSeq = 0;
      nextSeq = 0;
      resetStability();
      void refresh();
      resume();
    },
    { immediate: true },
  );

  watch(
    () => toValue(done),
    (isDone) => {
      if (isDone) {
        // Reset the stability window and fire an immediate refresh so we
        // catch identities that landed right around the "done" flip without
        // waiting a full slow-cadence tick.
        resetStability();
        void refresh();
        resume();
      } else if (toValue(pipelineId)) {
        resume();
      }
    },
  );

  onUnmounted(pause);

  return {
    workerCount,
    error,
    refresh,
  };
}
