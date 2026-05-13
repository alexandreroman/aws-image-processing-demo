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
      <a
        v-for="item in completed"
        :key="item.workflowId"
        :href="item.largeUrl"
        target="_blank"
        rel="noopener noreferrer"
        class="group block animate-fade-in"
        :title="item.description || item.imageId"
      >
        <div
          class="aspect-square overflow-hidden rounded-md bg-surface-hover
            border border-surface-border group-hover:border-primary/60
            transition-colors"
        >
          <img
            :src="item.thumbUrl"
            :alt="item.description || item.imageId"
            loading="lazy"
            class="h-full w-full object-cover transition-transform duration-300
              group-hover:scale-105"
          >
        </div>
      </a>

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
  </section>
</template>
