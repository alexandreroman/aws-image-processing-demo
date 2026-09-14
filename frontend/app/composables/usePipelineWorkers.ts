// Poll a pipeline's distinct worker count. Polls every 3s while the pipeline
// is running. Once `done` flips true we fire one delayed refresh (to catch
// identities that land seconds after the apparent completion on Lambda
// bursts) and then stop. The workerCount stays `null` until the first
// successful fetch so callers can render a skeleton.

import { useIntervalFn } from '@vueuse/core';

const POLL_MS = 3_000;
const POST_DONE_REFRESH_DELAY_MS = 5_000;

export function usePipelineWorkers(
  pipelineId: MaybeRefOrGetter<string>,
  done: MaybeRefOrGetter<boolean>,
): { workerCount: Readonly<Ref<number | null>> } {
  const api = useApi();

  const count = ref<number | null>(null);

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
      // The page component is reused across /pipelines/A -> /pipelines/B, so a
      // reply for the pipeline we just left must not land on the new one.
      if (id !== toValue(pipelineId)) return;
      if (seq <= lastAppliedSeq) return;
      lastAppliedSeq = seq;
      count.value = result.workerCount;
    } catch {
      // Cosmetic counter: keep the last known value and let the next poll retry.
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

  return { workerCount: count };
}
