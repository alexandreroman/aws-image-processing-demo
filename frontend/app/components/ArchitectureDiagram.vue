<script setup lang="ts">
// Animates a typical workflow execution on a 14-tick loop. Every number below
// is the tick at which the corresponding box turns green.
interface Activity {
  key: string;
  /** Vertical centre of the activity row in the SVG viewBox. */
  y: number;
  label: string;
  /** Ticks for the S/M/L sub-boxes, for activities that fan out per size. */
  subs?: [number, number, number];
  /** Badge for the external service the activity calls, and its tick. */
  service?: { label: string; tick: number };
  /** Tick at which the activity's check mark completes. */
  step: number;
}

const activities: Activity[] = [
  { key: 'resize', y: 105, label: 'Resize', subs: [1, 2, 3], step: 4 },
  { key: 'describe', y: 155, label: 'Describe with AI', service: { label: 'AI', tick: 5 }, step: 6 },
  { key: 'watermark', y: 205, label: 'Watermark', subs: [7, 8, 9], step: 10 },
  { key: 'store', y: 255, label: 'Store results', service: { label: 'DB', tick: 11 }, step: 12 },
];

const tick = ref(0);
let timer: ReturnType<typeof setInterval> | null = null;

function isDone(atTick: number | undefined): boolean {
  return atTick !== undefined && tick.value >= atTick;
}

// The branch arrow flows from the activity's first sub-step (or its service
// call, for activities that have no sub-steps) until the activity completes.
function arrowActive(a: Activity): boolean {
  const start = a.subs?.[0] ?? a.service?.tick ?? a.step;
  return tick.value >= start && tick.value <= a.step;
}

onMounted(() => {
  const reduced =
    typeof window !== 'undefined' &&
    window.matchMedia('(prefers-reduced-motion: reduce)').matches;
  if (reduced) {
    tick.value = 13;
    return;
  }
  timer = setInterval(() => {
    tick.value = (tick.value + 1) % 14;
  }, 600);
});

onBeforeUnmount(() => {
  if (timer) clearInterval(timer);
});
</script>

<template>
  <section class="card p-5 sm:p-6">
    <h2 class="stat-label mb-4">Workflow</h2>

    <svg
      viewBox="0 80 1080 210"
      class="w-full h-auto select-none"
      role="img"
      aria-label="Workflow execution through Resize, Describe with AI, Watermark, and Store results"
    >
      <!-- nodes -->
      <g class="text-ink-100" font-family="ui-sans-serif, system-ui" font-size="14">
        <g>
          <rect
            x="20" y="160" width="120" height="40" rx="8"
            class="fill-surface-elevated stroke-surface-border" stroke-width="1"/>
          <text x="80" y="184" text-anchor="middle" class="fill-ink-100">Browser</text>
        </g>
        <g>
          <rect
            x="200" y="160" width="120" height="40" rx="8"
            class="fill-surface-elevated stroke-accent" stroke-width="1.5"/>
          <text x="260" y="184" text-anchor="middle" class="fill-accent font-semibold">
            App
          </text>
        </g>
        <g>
          <rect
            x="380" y="160" width="170" height="40" rx="8"
            class="fill-surface-elevated stroke-primary" stroke-width="1.5"/>
          <text x="465" y="184" text-anchor="middle" class="fill-primary font-semibold">
            Temporal Cloud
          </text>
        </g>
        <g>
          <rect
            x="610" y="160" width="120" height="40" rx="8"
            class="fill-surface-elevated stroke-accent" stroke-width="1.5"/>
          <text x="670" y="184" text-anchor="middle" class="fill-accent font-semibold">
            Workers
          </text>
        </g>

        <!-- activity branches -->
        <g v-for="a in activities" :key="a.key">
          <rect
            x="820" :y="a.y - 16" width="200" height="32" rx="6"
            class="fill-surface stroke-surface-border" stroke-width="1"/>
          <text x="832" :y="a.y + 4" class="fill-ink-200" font-size="12">
            {{ a.label }}
          </text>

          <template v-if="a.subs">
            <g v-for="(sx, si) in [922, 954, 986]" :key="si">
              <rect
                :x="sx" :y="a.y - 10" width="28" height="20" rx="4"
                :class="isDone(a.subs?.[si])
                  ? 'fill-emerald-500/20 stroke-emerald-400'
                  : 'fill-surface stroke-surface-border'"
                stroke-width="1"
                style="transition: fill 220ms ease, stroke 220ms ease"
              />
              <text
                :x="sx + 14" :y="a.y + 4" text-anchor="middle" font-size="11"
                :class="isDone(a.subs?.[si]) ? 'fill-emerald-300' : 'fill-ink-300'"
                style="transition: fill 220ms ease"
              >
                {{ ['S', 'M', 'L'][si] }}
              </text>
            </g>
          </template>

          <template v-if="a.service">
            <rect
              x="986" :y="a.y - 10" width="28" height="20" rx="4"
              :class="isDone(a.service.tick)
                ? 'fill-emerald-500/20 stroke-emerald-400'
                : 'fill-surface stroke-surface-border'"
              stroke-width="1"
              style="transition: fill 220ms ease, stroke 220ms ease"
            />
            <text
              x="1000" :y="a.y + 4" text-anchor="middle" font-size="11"
              :class="isDone(a.service.tick) ? 'fill-emerald-300' : 'fill-ink-300'"
              style="transition: fill 220ms ease"
            >
              {{ a.service.label }}
            </text>
          </template>

          <g class="text-emerald-500" :class="{ 'is-done': isDone(a.step) }">
            <circle
              cx="1040" :cy="a.y" r="10"
              fill="none" stroke="currentColor" stroke-width="2"
              pathLength="1"
              class="check-circle"
            />
            <path
              :d="`M 1035 ${a.y} L 1038.5 ${a.y + 3.5} L 1045 ${a.y - 3.5}`"
              fill="none" stroke="currentColor" stroke-width="2"
              stroke-linecap="round" stroke-linejoin="round"
              pathLength="1"
              class="check-mark"
            />
          </g>

          <!-- branch arrow -->
          <path
            :d="`M 730 180 C 775 180, 775 ${a.y}, 820 ${a.y}`"
            :class="['arrow', arrowActive(a) ? 'text-ink-300' : 'arrow--idle text-surface-border']"
            fill="none"
            stroke="currentColor"
            stroke-width="2"
          />
        </g>
      </g>

      <!-- main horizontal arrows -->
      <g fill="none" stroke="currentColor" stroke-width="2" class="text-ink-300">
        <path d="M 140 180 L 200 180" class="arrow" style="animation-delay: 0s"/>
        <path d="M 320 180 L 380 180" class="arrow" style="animation-delay: 0.15s"/>
        <path d="M 550 180 L 610 180" class="arrow" style="animation-delay: 0.3s"/>
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

.arrow--idle {
  stroke-dasharray: none;
  animation: none;
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

/* pathLength="1" normalizes stroke length for dasharray math */
.check-circle,
.check-mark {
  stroke-dasharray: 1;
  stroke-dashoffset: 1;
}

.check-circle {
  transition: stroke-dashoffset 320ms ease;
}

.check-mark {
  transition: stroke-dashoffset 220ms ease 320ms;
}

.is-done .check-circle,
.is-done .check-mark {
  stroke-dashoffset: 0;
}

@media (prefers-reduced-motion: reduce) {
  .check-circle,
  .check-mark {
    transition: none;
  }
}
</style>
