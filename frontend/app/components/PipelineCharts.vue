<script setup lang="ts">
import { useNow } from '@vueuse/core';
import type { PipelineSummary, WorkflowItem } from '~/composables/useApi';

const props = defineProps<{
  workflows: WorkflowItem[];
  summary: PipelineSummary;
}>();

// Component is rendered under `ssr: false` on the pipeline page, so `useNow`
// (which relies on `window`) is safe. It ticks once per second so the
// trailing "running" data point keeps moving even between polls.
const now = useNow({ interval: 1_000 });

interface TimePoint {
  t: number; // seconds since t0
  running: number;
  completed: number;
  failed: number;
}

interface Series {
  points: TimePoint[];
  duration: number;
  yMax: number;
}

function parseTime(s: string | undefined): number | null {
  if (!s) return null;
  const ms = Date.parse(s);
  return Number.isFinite(ms) ? ms : null;
}

// Walk every start/terminal event in chronological order to derive the
// running/completed/failed series. Events are sorted by timestamp so each
// step yields a strictly monotonic `t` and a well-defined Y at that instant.
const series = computed<Series>(() => {
  const events: { ms: number; kind: 'start' | 'complete' | 'fail' }[] = [];

  for (const w of props.workflows) {
    const start = parseTime(w.startedAt);
    if (start !== null) events.push({ ms: start, kind: 'start' });

    const end = parseTime(w.completedAt);
    if (end !== null) {
      events.push({
        ms: end,
        kind: w.status === 'COMPLETED' ? 'complete' : 'fail',
      });
    }
  }

  if (events.length === 0) {
    return { points: [], duration: 0, yMax: Math.max(1, props.summary.total) };
  }

  events.sort((a, b) => a.ms - b.ms);
  const t0 = events[0]!.ms;

  const points: TimePoint[] = [{ t: 0, running: 0, completed: 0, failed: 0 }];
  let running = 0;
  let completed = 0;
  let failed = 0;

  for (const e of events) {
    if (e.kind === 'start') running += 1;
    else if (e.kind === 'complete') {
      running = Math.max(0, running - 1);
      completed += 1;
    } else {
      running = Math.max(0, running - 1);
      failed += 1;
    }
    points.push({ t: (e.ms - t0) / 1000, running, completed, failed });
  }

  // Anchor trailing point at "now" while workflows are still in flight so
  // the running line extends to the present and visibly advances each tick.
  if (props.summary.running > 0) {
    const lastT = Math.max(points[points.length - 1]!.t, (now.value.getTime() - t0) / 1000);
    points.push({ t: lastT, running, completed, failed });
  }

  const duration = points[points.length - 1]!.t;
  const yMax = Math.max(1, props.summary.total);
  return { points, duration, yMax };
});

const VB_W = 320;
const VB_H = 120;
const PAD_L = 4;
const PAD_R = 4;
const PAD_T = 6;
const PAD_B = 6;
const PLOT_W = VB_W - PAD_L - PAD_R;
const PLOT_H = VB_H - PAD_T - PAD_B;

function xScale(t: number, duration: number): number {
  if (duration <= 0) return PAD_L;
  return PAD_L + (t / duration) * PLOT_W;
}

function yScale(v: number, yMax: number): number {
  return PAD_T + PLOT_H - (v / yMax) * PLOT_H;
}

interface BuiltPaths {
  completedArea: string;
  failedArea: string;
  runningLine: string;
  targetY: number;
}

const paths = computed<BuiltPaths>(() => {
  const { points, duration, yMax } = series.value;
  if (points.length === 0) {
    return { completedArea: '', failedArea: '', runningLine: '', targetY: yScale(yMax, yMax) };
  }

  const baseY = yScale(0, yMax);

  // Step-after interpolation: counts change discretely on each event.
  const completedSteps: string[] = [];
  const failedSteps: string[] = [];
  const runningSteps: string[] = [];
  let prev = points[0]!;
  completedSteps.push(`M ${xScale(prev.t, duration)} ${baseY}`);
  failedSteps.push(`M ${xScale(prev.t, duration)} ${yScale(prev.completed, yMax)}`);
  runningSteps.push(`M ${xScale(prev.t, duration)} ${yScale(prev.running, yMax)}`);

  for (let i = 1; i < points.length; i += 1) {
    const p = points[i]!;
    const x = xScale(p.t, duration);

    completedSteps.push(`L ${x} ${yScale(prev.completed, yMax)}`);
    completedSteps.push(`L ${x} ${yScale(p.completed, yMax)}`);

    failedSteps.push(`L ${x} ${yScale(prev.completed, yMax)}`);
    failedSteps.push(`L ${x} ${yScale(p.completed + p.failed, yMax)}`);

    runningSteps.push(`L ${x} ${yScale(prev.running, yMax)}`);
    runningSteps.push(`L ${x} ${yScale(p.running, yMax)}`);

    prev = p;
  }

  const lastX = xScale(points[points.length - 1]!.t, duration);
  const completedArea = `${completedSteps.join(' ')} L ${lastX} ${baseY} Z`;

  // Close the failed ribbon by walking the completed line right-to-left so its
  // lower boundary hugs the top of the completed area. When `failed === 0` the
  // ribbon collapses to zero thickness instead of painting a triangle across
  // the chart back to (firstX, baseY).
  const failedReturn: string[] = [];
  failedReturn.push(`L ${lastX} ${yScale(points[points.length - 1]!.completed, yMax)}`);
  for (let i = points.length - 1; i >= 1; i -= 1) {
    const p = points[i]!;
    const prevP = points[i - 1]!;
    const prevX = xScale(prevP.t, duration);
    const x = xScale(p.t, duration);
    failedReturn.push(`L ${x} ${yScale(prevP.completed, yMax)}`);
    failedReturn.push(`L ${prevX} ${yScale(prevP.completed, yMax)}`);
  }
  const failedArea = `${failedSteps.join(' ')} ${failedReturn.join(' ')} Z`;
  const runningLine = runningSteps.join(' ');

  return { completedArea, failedArea, runningLine, targetY: yScale(yMax, yMax) };
});

const hasData = computed(() => series.value.points.length > 0);

type StageKey = 'ResizeAndUpload' | 'GenerateDescription' | 'ApplyWatermark' | 'StoreManifest' | 'Queued';

interface StageRow {
  key: StageKey;
  label: string;
  count: number;
}

const stages = computed<StageRow[]>(() => {
  const buckets: Record<StageKey, number> = {
    ResizeAndUpload: 0,
    GenerateDescription: 0,
    ApplyWatermark: 0,
    StoreManifest: 0,
    Queued: 0,
  };

  for (const w of props.workflows) {
    if (w.status !== 'RUNNING' && w.status !== 'CONTINUED_AS_NEW') continue;
    const a = w.currentActivity;
    if (a === 'ResizeAndUpload' || a === 'GenerateDescription'
      || a === 'ApplyWatermark' || a === 'StoreManifest') {
      buckets[a] += 1;
    } else {
      // Backend caps currentActivity lookups at 10 per poll, so unreported
      // running workflows are surfaced as "Queued" rather than hidden.
      buckets.Queued += 1;
    }
  }

  return [
    { key: 'Queued', label: 'Queued', count: buckets.Queued },
    { key: 'ResizeAndUpload', label: 'Resize', count: buckets.ResizeAndUpload },
    { key: 'GenerateDescription', label: 'Describe', count: buckets.GenerateDescription },
    { key: 'ApplyWatermark', label: 'Watermark', count: buckets.ApplyWatermark },
    { key: 'StoreManifest', label: 'Store', count: buckets.StoreManifest },
  ];
});

// Scale stage bars against `summary.running` so the X axis stays stable
// across polls; falling back to the max bucket if running is zero.
const stageScale = computed(() => Math.max(1, props.summary.running, ...stages.value.map((s) => s.count)));

function barWidthPct(count: number): string {
  return `${(count / stageScale.value) * 100}%`;
}

// Running uses `text-iris-400` (not `text-primary` like ControlPanel) so the
// counter color matches the iris line in the chart above, which is what lets
// us drop the legend without losing the color-coding cue.
const summaryRows = computed(() => [
  { label: 'Total', value: props.summary.total, color: 'text-ink-100' },
  { label: 'Running', value: props.summary.running, color: 'text-iris-400' },
  { label: 'Completed', value: props.summary.completed, color: 'text-emerald-400' },
  { label: 'Failed', value: props.summary.failed, color: 'text-rose-400' },
]);
</script>

<template>
  <section class="space-y-4">
    <article class="card p-4 space-y-3 animate-fade-in">
      <header>
        <h2 class="stat-label">Pipeline timeline</h2>
      </header>

      <div class="relative">
        <svg
          :viewBox="`0 0 ${VB_W} ${VB_H}`"
          preserveAspectRatio="none"
          class="w-full h-40 block"
          role="img"
          aria-label="Pipeline timeline chart"
        >
          <line
            v-if="hasData"
            :x1="PAD_L"
            :x2="VB_W - PAD_R"
            :y1="paths.targetY"
            :y2="paths.targetY"
            stroke="currentColor"
            class="text-ink-500"
            stroke-width="0.5"
            stroke-dasharray="2 2"
            vector-effect="non-scaling-stroke"
          />
          <path
            v-if="paths.completedArea"
            :d="paths.completedArea"
            fill="rgb(52 211 153 / 0.35)"
            stroke="rgb(52 211 153)"
            stroke-width="1"
            vector-effect="non-scaling-stroke"
          />
          <path
            v-if="paths.failedArea"
            :d="paths.failedArea"
            fill="rgb(244 63 94 / 0.35)"
          />
          <path
            v-if="paths.runningLine"
            :d="paths.runningLine"
            fill="none"
            stroke="rgb(167 139 250)"
            stroke-width="1.5"
            stroke-linejoin="round"
            vector-effect="non-scaling-stroke"
          />
        </svg>

        <div
          v-if="!hasData"
          class="pointer-events-none absolute inset-0 flex items-center justify-center"
          aria-hidden="true"
        >
          <div class="h-5 w-5 rounded-full border-2 border-primary/30 border-t-primary animate-spin" />
        </div>
      </div>

      <dl class="grid grid-cols-2 gap-1.5">
        <div
          v-for="row in summaryRows"
          :key="row.label"
          class="flex items-center justify-between rounded-md bg-surface-hover/60
            px-2.5 py-1.5"
        >
          <span class="text-[11px] text-ink-300">{{ row.label }}</span>
          <span
            :class="['font-mono font-semibold tabular-nums text-sm', row.color]"
          >
            {{ row.value }}
          </span>
        </div>
      </dl>
    </article>

    <article class="card p-4 space-y-3 animate-fade-in">
      <header class="flex items-baseline justify-between">
        <h2 class="stat-label">Active activity stages</h2>
        <span class="text-[11px] text-ink-400 font-mono tabular-nums">
          {{ summary.running }} running
        </span>
      </header>

      <ul class="space-y-2">
        <li
          v-for="row in stages"
          :key="row.key"
          class="grid grid-cols-[5.5rem_1fr_2.5rem] items-center gap-3"
        >
          <span class="text-[11px] text-ink-200">{{ row.label }}</span>
          <div class="h-2 rounded-full bg-surface-hover overflow-hidden">
            <div
              class="h-full rounded-full transition-all duration-300 ease-out"
              :class="row.key === 'Queued'
                ? 'bg-gradient-to-r from-ink-500 to-ink-400'
                : 'bg-gradient-to-r from-primary to-iris'"
              :style="{ width: barWidthPct(row.count) }"
              aria-hidden="true"
            />
          </div>
          <span class="font-mono text-sm tabular-nums text-right text-ink-100">
            {{ row.count }}
          </span>
        </li>
      </ul>
    </article>
  </section>
</template>
