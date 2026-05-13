<script setup lang="ts">
const { toasts, dismiss } = useToast();

function colorFor(kind: 'info' | 'success' | 'error'): string {
  switch (kind) {
    case 'success':
      return 'border-emerald-300 bg-emerald-50 text-emerald-900';
    case 'error':
      return 'border-rose-300 bg-rose-50 text-rose-900';
    default:
      return 'border-primary-200 bg-primary-50 text-primary-900';
  }
}
</script>

<template>
  <Teleport to="body">
    <div
      class="fixed top-4 right-4 z-50 flex flex-col gap-2 max-w-sm w-full
        pointer-events-none"
      aria-live="polite"
      aria-atomic="false"
    >
      <div
        v-for="t in toasts"
        :key="t.id"
        :class="[
          'pointer-events-auto rounded-md border shadow-card px-4 py-3',
          'animate-slide-in-right',
          colorFor(t.kind),
        ]"
        role="status"
      >
        <div class="flex items-start gap-3">
          <div class="flex-1 min-w-0">
            <p class="text-sm font-semibold">{{ t.title }}</p>
            <p v-if="t.body" class="text-xs mt-0.5 opacity-80">
              {{ t.body }}
            </p>
          </div>
          <button
            type="button"
            class="text-current/60 hover:text-current text-sm leading-none"
            aria-label="Dismiss"
            @click="dismiss(t.id)"
          >
            &times;
          </button>
        </div>
      </div>
    </div>
  </Teleport>
</template>
