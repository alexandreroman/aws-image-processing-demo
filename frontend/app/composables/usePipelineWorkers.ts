// Poll a pipeline's distinct worker count every 3s while the pipeline is
// running. When `done` flips true, switch to a slower 5s cadence and keep
// polling until the returned worker count has been identical for N
// consecutive successful polls (or until a 120s hard cap elapses), then
// pause. This catches identities that land seconds after the apparent
// completion on larger Lambda bursts without forcing a manual refresh.
// The workerCount stays `null` until the first successful fetch so callers
// can render a skeleton.

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

  // Drop out-of-order responses. The pipeline page can pause/resume polling
  // and trigger a final refresh; without sequencing, a slow earlier response
  // could clobber a fresher value.
  let nextSeq = 0;
  let lastAppliedSeq = 0;

  // Post-done loop state. Tracked outside the watcher so we can reset on
  // pipelineId change / unmount.
  let postDoneTimer: ReturnType<typeof setTimeout> | null = null;
  let postDoneDeadline = 0;
  let stableCount = 0;
  let lastStableValue: number | null = null;

  function stopPostDoneLoop() {
    if (postDoneTimer !== null) {
      clearTimeout(postDoneTimer);
      postDoneTimer = null;
    }
    stableCount = 0;
    lastStableValue = null;
  }

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

  // Post-done loop: refresh, then decide whether to stop (stable or capped)
  // or schedule the next slow tick. We re-read `done` on each tick so a
  // flip back to running cancels the loop and lets the running-phase
  // interval take over again.
  async function postDoneTick() {
    postDoneTimer = null;
    if (!toValue(done)) {
      stopPostDoneLoop();
      return;
    }
    await refresh();
    const after = count.value;

    if (after !== null && after === lastStableValue) {
      stableCount += 1;
    } else {
      lastStableValue = after;
      stableCount = after === null ? 0 : 1;
    }

    if (stableCount >= POST_DONE_STABLE_POLLS) {
      stopPostDoneLoop();
      return;
    }
    if (Date.now() >= postDoneDeadline) {
      stopPostDoneLoop();
      return;
    }
    postDoneTimer = setTimeout(() => {
      void postDoneTick();
    }, POST_DONE_POLL_MS);
  }

  function startPostDoneLoop() {
    stopPostDoneLoop();
    postDoneDeadline = Date.now() + POST_DONE_MAX_MS;
    // Fire an immediate refresh to capture identities that landed between
    // the last running-phase poll and the "done" flip, then enter the
    // slow cadence.
    void (async () => {
      await refresh();
      lastStableValue = count.value;
      stableCount = 1;
      postDoneTimer = setTimeout(() => {
        void postDoneTick();
      }, POST_DONE_POLL_MS);
    })();
  }

  watch(
    () => toValue(pipelineId),
    (id) => {
      pause();
      stopPostDoneLoop();
      if (!id) {
        return;
      }
      count.value = null;
      lastAppliedSeq = 0;
      nextSeq = 0;
      void refresh();
      if (toValue(done)) {
        startPostDoneLoop();
      } else {
        resume();
      }
    },
    { immediate: true },
  );

  watch(
    () => toValue(done),
    (isDone) => {
      if (isDone) {
        // Keep the running-phase interval paused while the slower
        // post-done loop drives polling.
        pause();
        startPostDoneLoop();
      } else {
        stopPostDoneLoop();
        if (toValue(pipelineId)) {
          resume();
        }
      }
    },
  );

  onUnmounted(() => {
    pause();
    stopPostDoneLoop();
  });

  return {
    workerCount,
    error,
    refresh,
  };
}
