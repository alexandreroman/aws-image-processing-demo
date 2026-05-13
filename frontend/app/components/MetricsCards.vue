<script setup lang="ts">
import type { SessionSummary } from '~/composables/useApi';

const props = defineProps<{
  summary: SessionSummary;
}>();

interface CardSpec {
  key: keyof SessionSummary;
  label: string;
  text: string;
  bar: string;
  ring: string;
}

const cards: CardSpec[] = [
  {
    key: 'total',
    label: 'Total',
    text: 'text-ink-100',
    bar: 'from-ink-300 to-ink-400',
    ring: 'ring-surface-border',
  },
  {
    key: 'running',
    label: 'Running',
    text: 'text-primary',
    bar: 'from-primary to-iris',
    ring: 'ring-primary/30',
  },
  {
    key: 'completed',
    label: 'Completed',
    text: 'text-emerald-400',
    bar: 'from-emerald-400 to-teal-300',
    ring: 'ring-emerald-500/30',
  },
  {
    key: 'failed',
    label: 'Failed',
    text: 'text-rose-400',
    bar: 'from-rose-500 to-accent',
    ring: 'ring-rose-500/30',
  },
];

function value(key: keyof SessionSummary): number {
  return props.summary[key];
}
</script>

<template>
  <section
    class="grid grid-cols-2 lg:grid-cols-2 xl:grid-cols-4 gap-3"
    aria-label="Workflow metrics"
  >
    <article
      v-for="c in cards"
      :key="c.key"
      :class="[
        'card relative overflow-hidden px-4 py-3 ring-1',
        c.ring,
      ]"
    >
      <div
        :class="[
          'absolute inset-x-0 top-0 h-0.5 bg-gradient-to-r',
          c.bar,
        ]"
        aria-hidden="true"
      />
      <div class="stat-label">{{ c.label }}</div>
      <div
        :class="[
          'mt-1 font-mono text-3xl font-bold tabular-nums leading-none',
          c.text,
        ]"
      >
        {{ value(c.key) }}
      </div>
    </article>
  </section>
</template>
