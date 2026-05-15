<script setup lang="ts">
const api = useApi();

const stats = ref<Stats | null>(null);
let timer: ReturnType<typeof setInterval> | null = null;

async function refresh() {
  try {
    stats.value = await api.getStats();
  } catch {
    // Cosmetic counters: swallow the error and keep the last value.
    // First-load failures leave stats null → the template renders dashes.
  }
}

onMounted(() => {
  refresh();
  timer = setInterval(refresh, 5000);
});

onBeforeUnmount(() => {
  if (timer !== null) clearInterval(timer);
});

const DASH = '—';

function formatCount(value: number | undefined): string {
  if (value === undefined || value === -1) return DASH;
  return value.toLocaleString('en-US');
}

function formatActivities(processed: number | undefined): string {
  if (processed === undefined || processed === -1) return DASH;
  return (processed * 8).toLocaleString('en-US');
}

function formatSuccessRate(
  processed: number | undefined,
  failed: number | undefined,
): string {
  if (processed === undefined || processed === -1) return DASH;
  if (failed === undefined || failed === -1) return DASH;
  const total = processed + failed;
  if (total === 0) return DASH;
  const pct = (processed / total) * 100;
  return `${pct.toFixed(1)}%`;
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
    <div class="flex items-baseline justify-between mb-4">
      <h2 class="stat-label">Live activity</h2>
      <span class="text-[11px] text-ink-400">
        Last {{ stats?.windowDays ?? 30 }} days
      </span>
    </div>
    <dl class="grid grid-cols-1 sm:grid-cols-2 lg:grid-cols-4 gap-4">
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
