// Poll a pipeline's distinct worker count. Polls every 3s while the pipeline
// is running. Once `done` flips true we fire one delayed refresh (to catch
// identities that land seconds after the apparent completion on Lambda
// bursts) and then stop. The workerCount stays `null` until the first
// successful fetch so callers can render a skeleton.

import { useIntervalFn } from '@vueuse/core';

const POLL_MS = 3_000;
const POST_DONE_REFRESH_DELAY_MS = 5_000;

export interface UsePipelineWorkersReturn {
  workerCount: Readonly<Ref<number | null>>;
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

  // Drop out-of-order responses: pause/resume + manual refresh can otherwise
  // let a slow earlier reply clobber a fresher value.
  let nextSeq = 0;
  let lastAppliedSeq = 0;

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

  const { pause, resume } = useIntervalFn(refresh, POLL_MS, {
    immediate: false,
    immediateCallback: false,
  });

  let postDoneTimer: ReturnType<typeof setTimeout> | null = null;
  function clearPostDoneTimer() {
    if (postDoneTimer !== null) {
      clearTimeout(postDoneTimer);
      postDoneTimer = null;
    }
  }

  watch(
    () => toValue(pipelineId),
    (id) => {
      pause();
      clearPostDoneTimer();
      if (!id) return;
      count.value = null;
      lastAppliedSeq = 0;
      nextSeq = 0;
      void refresh();
      if (!toValue(done)) resume();
    },
    { immediate: true },
  );

  watch(
    () => toValue(done),
    (isDone) => {
      if (!isDone) {
        clearPostDoneTimer();
        if (toValue(pipelineId)) resume();
        return;
      }
      pause();
      clearPostDoneTimer();
      // One last refresh after a short delay so we catch identities that
      // landed right around the "done" flip.
      postDoneTimer = setTimeout(() => {
        void refresh();
        postDoneTimer = null;
      }, POST_DONE_REFRESH_DELAY_MS);
    },
  );

  onUnmounted(() => {
    pause();
    clearPostDoneTimer();
  });

  return {
    workerCount: count,
    error,
    refresh,
  };
}
