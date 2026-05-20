<script setup lang="ts">
const { toasts, dismiss } = useToast();

function colorFor(kind: 'success' | 'error'): string {
  return kind === 'success'
    ? 'border-emerald-500/40 bg-emerald-500/10 text-emerald-100'
    : 'border-rose-500/40 bg-rose-500/10 text-rose-100';
}
</script>

<template>
  <Teleport to="body">
    <div
      class="fixed bottom-4 right-4 z-50 flex flex-col gap-2 max-w-sm w-full
        pointer-events-none"
      aria-live="polite"
      aria-atomic="false"
    >
      <div
        v-for="t in toasts"
        :key="t.id"
        :class="[
          'pointer-events-auto rounded-lg border backdrop-blur-md',
          'shadow-card px-4 py-3 animate-slide-in-right bg-surface/80',
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
            class="text-current/60 hover:text-current text-base leading-none"
            aria-label="Dismiss"
            @click="dismiss(t.id)"
          >
            ×
          </button>
        </div>
      </div>
    </div>
  </Teleport>
</template>
