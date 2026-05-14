<script setup lang="ts">
import type { WorkflowItem } from '~/composables/useApi';

const props = defineProps<{
  workflows: WorkflowItem[];
}>();

const config = useRuntimeConfig();

function publicUrl(bucket: string, key: string): string {
  const base = config.public.s3PublicUrl.replace(/\/$/, '');
  return `${base}/${bucket}/${key}`;
}

interface CompletedThumb {
  workflowId: string;
  imageId: string;
  description: string;
  labels: string[];
  thumbUrl: string;
  largeUrl: string;
}

const completed = computed<CompletedThumb[]>(() =>
  props.workflows
    .filter((w) => w.status === 'COMPLETED' && w.manifest)
    .map((w) => {
      const m = w.manifest!;
      const medium = m.watermarked?.medium ?? m.sizes.medium?.s3Ref;
      const large = m.watermarked?.large ?? m.sizes.large?.s3Ref;
      const ref = medium ?? m.original;
      const big = large ?? m.original;
      return {
        workflowId: w.workflowId,
        imageId: m.imageId,
        description: m.description ?? '',
        labels: m.labels ?? [],
        thumbUrl: publicUrl(ref.bucket, ref.key),
        largeUrl: publicUrl(big.bucket, big.key),
      };
    }),
);

const running = computed(() =>
  props.workflows.filter((w) => w.status === 'RUNNING'),
);

const failed = computed(() =>
  props.workflows.filter(
    (w) =>
      w.status === 'FAILED' ||
      w.status === 'TIMED_OUT' ||
      w.status === 'TERMINATED',
  ),
);

const selected = ref<CompletedThumb | null>(null);

function openModal(item: CompletedThumb): void {
  selected.value = item;
}

function closeModal(): void {
  selected.value = null;
}

function onKeydown(e: KeyboardEvent): void {
  if (e.key === 'Escape') closeModal();
}

// Lock body scroll + bind Escape only while a modal is open. Guarded for SSG.
watch(selected, (item) => {
  if (typeof document === 'undefined') return;
  if (item) {
    document.documentElement.classList.add('overflow-hidden');
    document.addEventListener('keydown', onKeydown);
  } else {
    document.documentElement.classList.remove('overflow-hidden');
    document.removeEventListener('keydown', onKeydown);
  }
});

onBeforeUnmount(() => {
  if (typeof document === 'undefined') return;
  document.documentElement.classList.remove('overflow-hidden');
  document.removeEventListener('keydown', onKeydown);
});
</script>

<template>
  <section class="card p-4 space-y-3">
    <header class="flex items-baseline justify-between">
      <h2 class="stat-label">Gallery</h2>
      <span class="text-[11px] text-ink-400 font-mono tabular-nums">
        {{ completed.length }} / {{ workflows.length }}
      </span>
    </header>

    <div
      v-if="workflows.length === 0"
      class="text-xs text-ink-400 py-10 text-center border border-dashed
        border-surface-border rounded-lg"
    >
      Workflows will appear here as they complete.
    </div>

    <div
      v-else
      class="grid grid-cols-3 sm:grid-cols-4 gap-2"
    >
      <button
        v-for="item in completed"
        :key="item.workflowId"
        type="button"
        class="group block animate-fade-in text-left
          focus-visible:outline-none focus-visible:ring-2
          focus-visible:ring-primary/60 focus-visible:ring-offset-2
          focus-visible:ring-offset-bg rounded-md"
        :title="item.description || item.imageId"
        :aria-label="`Open ${item.description || item.imageId}`"
        @click="openModal(item)"
      >
        <div
          class="aspect-square overflow-hidden rounded-md bg-surface-hover
            border border-surface-border transition-all duration-300
            group-hover:border-primary group-hover:ring-2
            group-hover:ring-primary/60 group-hover:shadow-glow
            group-hover:scale-[1.03] relative"
        >
          <img
            :src="item.thumbUrl"
            :alt="item.description || item.imageId"
            loading="lazy"
            class="h-full w-full object-cover transition-transform duration-300
              group-hover:scale-105"
          >
          <div
            class="absolute inset-0 bg-gradient-to-t from-bg/60 to-transparent
              opacity-0 group-hover:opacity-100 transition-opacity duration-300"
            aria-hidden="true"
          />
        </div>
      </button>

      <div
        v-for="w in running"
        :key="w.workflowId"
        class="animate-fade-in"
        :title="`Running: ${w.currentActivity ?? '…'}`"
      >
        <div
          class="aspect-square rounded-md bg-gradient-to-br from-primary/10
            to-iris/10 border border-primary/30 flex items-center
            justify-center animate-pulse-glow"
        >
          <div
            class="h-5 w-5 rounded-full border-2 border-primary/30 border-t-primary
              animate-spin"
            aria-hidden="true"
          />
        </div>
      </div>

      <div
        v-for="w in failed"
        :key="w.workflowId"
        class="animate-fade-in"
        :title="w.status"
      >
        <div
          class="aspect-square rounded-md bg-rose-500/10 border
            border-rose-500/40 flex items-center justify-center text-rose-400
            text-2xl font-bold"
          aria-hidden="true"
        >
          ×
        </div>
      </div>
    </div>

    <Teleport to="body">
      <div
        v-if="selected"
        class="fixed inset-0 z-50 flex items-center justify-center p-4
          bg-bg/80 backdrop-blur-md animate-fade-in"
        role="dialog"
        aria-modal="true"
        :aria-label="selected.description || selected.imageId"
        @click.self="closeModal"
      >
        <div
          class="card-elevated relative w-full max-w-5xl max-h-[90vh]
            overflow-y-auto p-4 sm:p-6"
        >
          <button
            type="button"
            class="absolute top-3 right-3 z-10 inline-flex h-9 w-9
              items-center justify-center rounded-full bg-surface/80
              border border-surface-border text-ink-200 hover:text-primary
              hover:border-primary/60 transition-colors text-2xl
              leading-none focus-visible:outline-none focus-visible:ring-2
              focus-visible:ring-primary/60"
            aria-label="Close"
            @click="closeModal"
          >
            ×
          </button>

          <div class="flex justify-center bg-bg/40 rounded-lg overflow-hidden">
            <img
              :src="selected.largeUrl"
              :alt="selected.description || selected.imageId"
              class="max-h-[80vh] max-w-full object-contain"
            >
          </div>

          <div class="mt-4 space-y-3">
            <p
              v-if="selected.description"
              class="text-sm text-ink-100 leading-relaxed"
            >
              {{ selected.description }}
            </p>
            <p
              v-else
              class="text-sm text-ink-400 italic"
            >
              No description available.
            </p>

            <div
              v-if="selected.labels.length > 0"
              class="flex flex-wrap gap-1.5"
            >
              <span
                v-for="label in selected.labels"
                :key="label"
                class="chip-accent"
              >
                {{ label }}
              </span>
            </div>
          </div>
        </div>
      </div>
    </Teleport>
  </section>
</template>
