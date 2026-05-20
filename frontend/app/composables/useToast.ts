// Tiny global toast bus. Components push toasts via success()/error();
// <EventToaster> consumes them. Backed by a single module-level reactive
// array so multiple instances stay in sync.

export type ToastKind = 'success' | 'error';

export interface Toast {
  id: number;
  kind: ToastKind;
  title: string;
  body?: string;
}

const TTL_MS = 4_000;

const toasts = ref<Toast[]>([]);
let nextId = 1;

export function useToast() {
  function push(kind: ToastKind, title: string, body?: string) {
    const id = nextId++;
    toasts.value = [...toasts.value, { id, kind, title, body }];
    if (import.meta.client) {
      window.setTimeout(() => dismiss(id), TTL_MS);
    }
  }

  function dismiss(id: number) {
    toasts.value = toasts.value.filter((t) => t.id !== id);
  }

  return {
    toasts,
    dismiss,
    success: (title: string, body?: string) => push('success', title, body),
    error: (title: string, body?: string) => push('error', title, body),
  };
}
