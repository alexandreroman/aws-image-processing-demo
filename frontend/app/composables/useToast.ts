// Tiny global toast bus. Components push toasts; <EventToaster> consumes
// them. Backed by a single module-level reactive array so multiple
// instances stay in sync.

export type ToastKind = 'info' | 'success' | 'error';

export interface Toast {
  id: number;
  kind: ToastKind;
  title: string;
  body?: string;
  ttlMs: number;
}

const DEFAULT_TTL_MS = 4_000;

const toasts = ref<Toast[]>([]);
let nextId = 1;

export function useToast() {
  function push(
    title: string,
    opts: { kind?: ToastKind; body?: string; ttlMs?: number } = {},
  ): number {
    const id = nextId++;
    const toast: Toast = {
      id,
      kind: opts.kind ?? 'info',
      title,
      body: opts.body,
      ttlMs: opts.ttlMs ?? DEFAULT_TTL_MS,
    };
    toasts.value = [...toasts.value, toast];
    if (import.meta.client) {
      window.setTimeout(() => dismiss(id), toast.ttlMs);
    }
    return id;
  }

  function dismiss(id: number) {
    toasts.value = toasts.value.filter((t) => t.id !== id);
  }

  function success(title: string, body?: string) {
    return push(title, { kind: 'success', body });
  }

  function error(title: string, body?: string) {
    return push(title, { kind: 'error', body });
  }

  function info(title: string, body?: string) {
    return push(title, { kind: 'info', body });
  }

  return { toasts, push, dismiss, success, error, info };
}
