<script setup lang="ts">
import type { Stats } from '~/composables/useApi';

const api = useApi();

// Point-in-time snapshot: the counters are fetched once on mount and never
// refreshed afterwards.
const stats = ref<Stats | null>(null);

onMounted(async () => {
  try {
    stats.value = await api.getStats();
  } catch {
    // Cosmetic counters: a failure just leaves `stats` null and the tiles
    // render dashes.
  }
});

const DASH = '—';
const ACTIVITIES_PER_IMAGE = 8;

// The backend reports `-1` for a counter it could not read; treat that and a
// missing value alike as "unknown".
function known(value: number | undefined): number | null {
  return value === undefined || value === -1 ? null : value;
}

function formatCount(value: number | undefined): string {
  const count = known(value);
  return count === null ? DASH : count.toLocaleString('en-US');
}

function formatActivities(processed: number | undefined): string {
  const count = known(processed);
  return count === null ? DASH : (count * ACTIVITIES_PER_IMAGE).toLocaleString('en-US');
}

function formatSuccessRate(
  processed: number | undefined,
  failed: number | undefined,
): string {
  const ok = known(processed);
  const ko = known(failed);
  if (ok === null || ko === null) return DASH;
  const total = ok + ko;
  if (total === 0) return DASH;
  return `${((ok / total) * 100).toFixed(1)}%`;
}

const tiles = computed(() => [
  {
    label: 'Bursts launched',
    value: formatCount(stats.value?.burstsLaunched),
  },
  {
    label: 'Images processed',
    value: formatCount(stats.value?.imagesProcessed),
  },
  {
    label: 'Activities executed',
    value: formatActivities(stats.value?.imagesProcessed),
  },
  {
    label: 'Success rate',
    value: formatSuccessRate(
      stats.value?.imagesProcessed,
      stats.value?.imagesFailed,
    ),
  },
]);
</script>

<template>
  <section class="card p-5 sm:p-6">
    <dl class="grid grid-cols-2 lg:grid-cols-4 gap-4">
      <div v-for="t in tiles" :key="t.label" class="min-w-0">
        <dt class="stat-label">{{ t.label }}</dt>
        <dd
          class="mt-1 text-2xl sm:text-3xl lg:text-4xl font-bold text-ink-100
            tabular-nums truncate"
          :title="t.value"
        >
          {{ t.value }}
        </dd>
      </div>
    </dl>
  </section>
</template>
