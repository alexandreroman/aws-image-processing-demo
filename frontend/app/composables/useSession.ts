// Poll a single session every 1.5s. Stops polling once everything is done
// (running === 0 && total > 0) or when the component using it unmounts.

import { useIntervalFn } from '@vueuse/core';
import type { Session, SessionSummary, WorkflowItem } from './useApi';

const POLL_INTERVAL_MS = 1_500;

export interface UseSessionReturn {
  session: Ref<Session | null>;
  summary: ComputedRef<SessionSummary>;
  workflows: ComputedRef<WorkflowItem[]>;
  loading: Ref<boolean>;
  error: Ref<Error | null>;
  isPolling: Ref<boolean>;
  refresh: () => Promise<void>;
  stop: () => void;
}

const EMPTY_SUMMARY: SessionSummary = {
  total: 0,
  running: 0,
  completed: 0,
  failed: 0,
};

export function useSession(sessionId: MaybeRefOrGetter<string>): UseSessionReturn {
  const api = useApi();

  const session = ref<Session | null>(null);
  const loading = ref(false);
  const error = ref<Error | null>(null);

  const summary = computed<SessionSummary>(
    () => session.value?.summary ?? EMPTY_SUMMARY,
  );
  const workflows = computed<WorkflowItem[]>(
    () => session.value?.workflows ?? [],
  );

  async function refresh() {
    const id = toValue(sessionId);
    if (!id) {
      return;
    }
    loading.value = true;
    try {
      session.value = await api.getSession(id);
      error.value = null;
    } catch (err) {
      error.value = err instanceof Error ? err : new Error(String(err));
    } finally {
      loading.value = false;
    }
  }

  const { pause, resume, isActive } = useIntervalFn(
    () => {
      void refresh();
    },
    POLL_INTERVAL_MS,
    { immediate: false, immediateCallback: false },
  );

  // Stop polling automatically when everything is done.
  watch(summary, (s) => {
    if (s.total > 0 && s.running === 0) {
      pause();
    }
  });

  watch(
    () => toValue(sessionId),
    (id) => {
      if (!id) {
        pause();
        return;
      }
      void refresh();
      resume();
    },
    { immediate: true },
  );

  onUnmounted(() => {
    pause();
  });

  return {
    session,
    summary,
    workflows,
    loading,
    error,
    isPolling: isActive,
    refresh,
    stop: pause,
  };
}
