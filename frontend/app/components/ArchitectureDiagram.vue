<script setup lang="ts">
// Animates a typical workflow execution on a 14-tick loop.
type Activity =
  | { y: number; key: 'resize'; label: string; subs: true }
  | { y: number; key: 'watermark'; label: string; subs: true }
  | { y: number; key: 'describe'; label: string; subs: false; service: string }
  | { y: number; key: 'store'; label: string; subs: false; service: string };

const activities: Activity[] = [
  { y: 105, key: 'resize', label: 'Resize', subs: true },
  { y: 155, key: 'describe', label: 'Describe with AI', subs: false, service: 'AI' },
  { y: 205, key: 'watermark', label: 'Watermark', subs: true },
  { y: 255, key: 'store', label: 'Store results', subs: false, service: 'DB' },
];

type StepKey = Activity['key'];

const completion: Record<
  StepKey,
  { subs: [number, number, number] | null; service?: number | null; step: number }
> = {
  resize: { subs: [1, 2, 3], step: 4 },
  describe: { subs: null, service: 5, step: 6 },
  watermark: { subs: [7, 8, 9], step: 10 },
  store: { subs: null, service: 11, step: 12 },
};

const activeTicks: Record<StepKey, [number, number]> = {
  resize: [1, 4],
  describe: [5, 6],
  watermark: [7, 10],
  store: [11, 12],
};

const tick = ref(0);
let timer: ReturnType<typeof setInterval> | null = null;

function subDone(stepKey: StepKey, idx: number): boolean {
  const subs = completion[stepKey].subs;
  return subs ? tick.value >= subs[idx]! : false;
}

function stepDone(stepKey: StepKey): boolean {
  return tick.value >= completion[stepKey].step;
}

function serviceDone(stepKey: StepKey): boolean {
  const service = completion[stepKey].service;
  return service != null && tick.value >= service;
}

function arrowActive(stepKey: StepKey): boolean {
  const [start, end] = activeTicks[stepKey];
  return tick.value >= start && tick.value <= end;
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
        <g class="node">
          <rect
x="20"  y="160" width="120" height="40" rx="8"
            class="fill-surface-elevated stroke-surface-border" stroke-width="1"/>
          <text x="80"  y="184" text-anchor="middle" class="fill-ink-100">Browser</text>
        </g>
        <g class="node">
          <rect
x="200" y="160" width="120" height="40" rx="8"
            class="fill-surface-elevated stroke-accent" stroke-width="1.5"/>
          <text x="260" y="184" text-anchor="middle" class="fill-accent font-semibold">
            App
          </text>
        </g>
        <g class="node">
          <rect
x="380" y="160" width="170" height="40" rx="8"
            class="fill-surface-elevated stroke-primary" stroke-width="1.5"/>
          <text x="465" y="184" text-anchor="middle" class="fill-primary font-semibold">
            Temporal Cloud
          </text>
        </g>
        <g class="node">
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
                :class="subDone(a.key, si)
                  ? 'fill-emerald-500/20 stroke-emerald-400'
                  : 'fill-surface stroke-surface-border'"
                stroke-width="1"
                style="transition: fill 220ms ease, stroke 220ms ease"
              />
              <text
                :x="sx + 14" :y="a.y + 4" text-anchor="middle" font-size="11"
                :class="subDone(a.key, si) ? 'fill-emerald-300' : 'fill-ink-300'"
                style="transition: fill 220ms ease"
              >
                {{ ['S', 'M', 'L'][si] }}
              </text>
            </g>
          </template>

          <template v-if="'service' in a">
            <rect
              x="986" :y="a.y - 10" width="28" height="20" rx="4"
              :class="serviceDone(a.key)
                ? 'fill-emerald-500/20 stroke-emerald-400'
                : 'fill-surface stroke-surface-border'"
              stroke-width="1"
              style="transition: fill 220ms ease, stroke 220ms ease"
            />
            <text
              x="1000" :y="a.y + 4" text-anchor="middle" font-size="11"
              :class="serviceDone(a.key) ? 'fill-emerald-300' : 'fill-ink-300'"
              style="transition: fill 220ms ease"
            >
              {{ a.service }}
            </text>
          </template>

          <g class="text-emerald-500" :class="{ 'is-done': stepDone(a.key) }">
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
            :class="['arrow', arrowActive(a.key) ? 'text-ink-300' : 'arrow--idle text-surface-border']"
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
