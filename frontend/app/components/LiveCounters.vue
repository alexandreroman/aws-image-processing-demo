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

function format(value: number | undefined): string {
  if (value === undefined || value === -1) return '—';
  return value.toLocaleString('en-US');
}

const tiles = computed(() => [
  { label: 'Images processed', value: format(stats.value?.imagesProcessed) },
  { label: 'Bursts launched',  value: format(stats.value?.burstsLaunched) },
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
    <dl class="grid grid-cols-1 sm:grid-cols-2 gap-4">
      <div v-for="t in tiles" :key="t.label" class="min-w-0">
        <dt class="stat-label">{{ t.label }}</dt>
        <dd
          class="mt-1 text-3xl sm:text-4xl font-bold text-ink-100 tabular-nums truncate"
          :title="t.value"
        >
          {{ t.value }}
        </dd>
      </div>
    </dl>
  </section>
</template>
