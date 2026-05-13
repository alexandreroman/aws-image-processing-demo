<script setup lang="ts">
import { useClipboard } from '@vueuse/core';

definePageMeta({
  // SPA fallback: rendered client-side only so the dynamic [id] can be
  // read at runtime. See `routeRules` in nuxt.config.ts.
  ssr: false,
});

const route = useRoute();
const toast = useToast();

const sessionId = computed(() => String(route.params.id ?? ''));

const { summary, workflows, error, isPolling, refresh } = useSession(
  sessionId,
);

useHead(() => ({
  title: `Session ${sessionId.value} — AWS Image Processing Demo`,
}));

// Toast when a new completion shows up.
const seenCompleted = new Set<string>();
let primed = false;
watch(
  workflows,
  (list) => {
    if (!primed) {
      // First payload: prime the seen set without firing toasts so we
      // do not spam when the user re-opens an already-finished session.
      for (const w of list) {
        if (w.status === 'COMPLETED') seenCompleted.add(w.workflowId);
      }
      primed = true;
      return;
    }
    for (const w of list) {
      if (w.status === 'COMPLETED' && !seenCompleted.has(w.workflowId)) {
        seenCompleted.add(w.workflowId);
        const desc = w.manifest?.description;
        toast.success('Image processed', desc ?? w.workflowId);
      }
    }
  },
  { deep: true },
);

// Shareable-URL banner.
const shareUrl = ref('');
onMounted(() => {
  shareUrl.value = window.location.href;
});
const clipboard = useClipboard({ source: shareUrl, legacy: true });

function copyShareUrl() {
  void clipboard.copy(shareUrl.value);
  toast.info('Link copied', 'Share it for the same live view.');
}
</script>

<template>
  <div class="max-w-7xl mx-auto px-4 sm:px-6 lg:px-8 py-6 space-y-6">
    <button
      type="button"
      class="w-full text-left card px-4 py-3 flex items-center gap-3
        hover:border-primary-200 transition-colors group"
      @click="copyShareUrl"
    >
      <span
        class="inline-flex items-center gap-2 chip bg-primary-50
          text-primary border border-primary-100"
      >
        Session
        <span class="font-mono">{{ sessionId }}</span>
      </span>
      <span class="flex-1 truncate text-sm text-ink-500 font-mono">
        {{ shareUrl }}
      </span>
      <span
        class="text-xs text-ink-400 group-hover:text-primary transition-colors"
      >
        Click to copy
      </span>
    </button>

    <div
      v-if="error"
      class="card border-rose-200 bg-rose-50 text-rose-800 px-4 py-3 text-sm"
    >
      <strong class="font-semibold">Couldn't load session:</strong>
      {{ error.message }}
      <button
        type="button"
        class="ml-2 underline"
        @click="() => refresh()"
      >
        Retry
      </button>
    </div>

    <div class="grid lg:grid-cols-12 gap-6">
      <aside class="lg:col-span-3 space-y-4">
        <ControlPanel :summary="summary" read-only />
        <div class="text-xs text-ink-400 px-1">
          <span v-if="isPolling">
            Polling every 1.5s…
          </span>
          <span v-else>
            Polling stopped (all done).
          </span>
        </div>
      </aside>

      <section class="lg:col-span-5 space-y-4">
        <MetricsCards :summary="summary" />
      </section>

      <section class="lg:col-span-4">
        <Gallery :workflows="workflows" />
      </section>
    </div>
  </div>
</template>
