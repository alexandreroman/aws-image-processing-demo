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
  <section class="card p-5 space-y-4">
    <header class="flex items-baseline justify-between">
      <h2 class="text-sm font-semibold text-ink-500 uppercase tracking-wide">
        Gallery
      </h2>
      <span class="text-xs text-ink-400">
        {{ completed.length }} of {{ workflows.length }} ready
      </span>
    </header>

    <div
      v-if="workflows.length === 0"
      class="text-sm text-ink-500 py-8 text-center"
    >
      Workflows will appear here as they complete.
    </div>

    <div
      v-else
      class="grid grid-cols-2 md:grid-cols-3 gap-3"
    >
      <a
        v-for="item in completed"
        :key="item.workflowId"
        :href="item.largeUrl"
        target="_blank"
        rel="noopener noreferrer"
        class="group block animate-fade-in"
      >
        <div
          class="aspect-square overflow-hidden rounded-md bg-ink-100 border
            border-ink-200 group-hover:border-primary-300 transition-colors"
        >
          <img
            :src="item.thumbUrl"
            :alt="item.description || item.imageId"
            loading="lazy"
            class="h-full w-full object-cover transition-transform
              duration-300 group-hover:scale-105"
          >
        </div>
        <p
          v-if="item.description"
          class="mt-1.5 text-xs text-ink-700 line-clamp-1"
          :title="item.description"
        >
          {{ item.description }}
        </p>
        <div v-if="item.labels.length" class="mt-1 flex flex-wrap gap-1">
          <span
            v-for="label in item.labels.slice(0, 3)"
            :key="label"
            class="chip"
          >
            {{ label }}
          </span>
        </div>
      </a>

      <div
        v-for="w in running"
        :key="w.workflowId"
        class="animate-fade-in"
        :title="`Running: ${w.currentActivity ?? '…'}`"
      >
        <div
          class="aspect-square rounded-md bg-gradient-to-br from-primary-50
            to-primary-100 border border-primary-200 flex items-center
            justify-center"
        >
          <div
            class="h-6 w-6 rounded-full border-2 border-primary-300
              border-t-primary animate-spin"
            aria-hidden="true"
          />
        </div>
        <p class="mt-1.5 text-xs text-ink-500 truncate">
          {{ w.currentActivity ?? 'Processing…' }}
        </p>
      </div>

      <div
        v-for="w in failed"
        :key="w.workflowId"
        class="animate-fade-in"
      >
        <div
          class="aspect-square rounded-md bg-rose-50 border border-rose-200
            flex items-center justify-center text-rose-500 text-3xl"
          aria-hidden="true"
        >
          &times;
        </div>
        <p class="mt-1.5 text-xs text-rose-600 truncate">
          {{ w.status }}
        </p>
      </div>
    </div>
  </section>
</template>
