<script setup lang="ts">
import type { SessionSummary } from '~/composables/useApi';

const props = defineProps<{
  summary: SessionSummary;
}>();

interface CardSpec {
  key: keyof SessionSummary;
  label: string;
  // Tailwind classes for the top border + number color.
  border: string;
  text: string;
  gradient: string;
}

const cards: CardSpec[] = [
  {
    key: 'total',
    label: 'Total',
    border: 'border-t-ink-400',
    text: 'text-ink-800',
    gradient: 'from-ink-50 to-white',
  },
  {
    key: 'running',
    label: 'Running',
    border: 'border-t-primary',
    text: 'text-primary',
    gradient: 'from-primary-50 to-white',
  },
  {
    key: 'completed',
    label: 'Completed',
    border: 'border-t-emerald-500',
    text: 'text-emerald-600',
    gradient: 'from-emerald-50 to-white',
  },
  {
    key: 'failed',
    label: 'Failed',
    border: 'border-t-rose-500',
    text: 'text-rose-600',
    gradient: 'from-rose-50 to-white',
  },
];

function value(key: keyof SessionSummary): number {
  return props.summary[key];
}
</script>

<template>
  <section
    class="grid grid-cols-2 lg:grid-cols-4 gap-4"
    aria-label="Workflow metrics"
  >
    <article
      v-for="c in cards"
      :key="c.key"
      :class="[
        'card border-t-4 px-5 py-4 bg-gradient-to-b',
        c.border,
        c.gradient,
      ]"
    >
      <div class="text-xs font-medium uppercase tracking-wide text-ink-500">
        {{ c.label }}
      </div>
      <div
        :class="['mt-1 font-mono text-4xl font-bold tabular-nums', c.text]"
      >
        {{ value(c.key) }}
      </div>
    </article>
  </section>
</template>
