<script setup lang="ts">
// Static conceptual diagram. The animation is ambient (CSS-driven)
// and intentionally decoupled from any live state — it teaches the
// reading order, not the current activity level.
const activities = [
  { y: 110, label: 'Resize' },
  { y: 170, label: 'Describe (Anthropic)' },
  { y: 230, label: 'Watermark' },
  { y: 290, label: 'Store' },
] as const;
</script>

<template>
  <section class="card p-5 sm:p-6">
    <h2 class="stat-label mb-4">Architecture</h2>

    <svg
      viewBox="0 0 1000 360"
      class="w-full h-auto select-none"
      role="img"
      aria-label="Browser to App to Temporal Cloud to Workers to four activities"
    >
      <!-- nodes -->
      <g class="text-ink-100" font-family="ui-sans-serif, system-ui" font-size="14">
        <g class="node">
          <rect x="20"  y="160" width="120" height="40" rx="8"
            class="fill-surface-elevated stroke-surface-border" stroke-width="1"/>
          <text x="80"  y="184" text-anchor="middle" class="fill-ink-100">Browser</text>
        </g>
        <g class="node">
          <rect x="220" y="160" width="120" height="40" rx="8"
            class="fill-surface-elevated stroke-surface-border" stroke-width="1"/>
          <text x="280" y="184" text-anchor="middle" class="fill-ink-100">App</text>
        </g>
        <g class="node">
          <rect x="420" y="160" width="170" height="40" rx="8"
            class="fill-surface-elevated stroke-primary" stroke-width="1.5"/>
          <text x="505" y="184" text-anchor="middle" class="fill-primary font-semibold">
            Temporal Cloud
          </text>
        </g>
        <g class="node">
          <rect x="670" y="160" width="120" height="40" rx="8"
            class="fill-surface-elevated stroke-accent" stroke-width="1.5"/>
          <text x="730" y="184" text-anchor="middle" class="fill-accent font-semibold">
            Workers
          </text>
        </g>

        <!-- activity branches -->
        <g v-for="(a, i) in activities" :key="a.label">
          <rect :x="850" :y="a.y - 16" width="130" height="32" rx="6"
            class="fill-surface stroke-surface-border" stroke-width="1"/>
          <text :x="915" :y="a.y + 4" text-anchor="middle" class="fill-ink-200" font-size="12">
            {{ a.label }}
          </text>
          <!-- branch arrow -->
          <path
            :d="`M 790 180 C 820 180, 820 ${a.y}, 850 ${a.y}`"
            class="arrow"
            :style="{ animationDelay: `${0.4 + i * 0.15}s` }"
            fill="none"
            stroke="currentColor"
            stroke-width="2"
          />
        </g>
      </g>

      <!-- main horizontal arrows -->
      <g fill="none" stroke="currentColor" stroke-width="2" class="text-ink-300">
        <path d="M 140 180 L 220 180" class="arrow" style="animation-delay: 0s"/>
        <path d="M 340 180 L 420 180" class="arrow" style="animation-delay: 0.15s"/>
        <path d="M 590 180 L 670 180" class="arrow" style="animation-delay: 0.3s"/>
      </g>
    </svg>
  </section>
</template>

<style scoped>
.arrow {
  stroke-dasharray: 6 6;
  stroke-dashoffset: 0;
  animation: dash 1.6s linear infinite;
}

@keyframes dash {
  to { stroke-dashoffset: -24; }
}

@media (prefers-reduced-motion: reduce) {
  .arrow {
    animation: none;
    stroke-dasharray: none;
  }
}
</style>
