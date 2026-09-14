<script setup lang="ts">
import type { WorkflowItem } from '~/composables/useApi';

const DEFAULT_PLACEHOLDER_SLOTS = 20;

const props = withDefaults(
  defineProps<{
    workflows: WorkflowItem[];
    expectedCount?: number;
  }>(),
  { expectedCount: 0 },
);

const config = useRuntimeConfig();

function publicUrl(key: string): string {
  const base = config.public.s3PublicUrl.replace(/\/$/, '');
  if (!base) {
    // Same-origin via reverse proxy (/images/* → S3 in prod, Moto in compose).
    return `/images/${key}`;
  }
  return `${base}/${key}`;
}

interface CompletedThumb {
  imageId: string;
  description: string;
  labels: string[];
  thumbUrl: string;
  largeUrl: string;
}

type TileStatus = 'completed' | 'running' | 'failed';

// image: when status='completed' the final variant, when status='running' an
// in-flight resized preview behind the spinner.
interface Tile {
  workflowId: string;
  status: TileStatus;
  image?: CompletedThumb;
  title: string;
}

type ThumbKind = 'watermarked' | 'resized';

interface CachedThumb {
  kind: ThumbKind;
  thumb: CompletedThumb;
}

// Why: a tile may first appear with the resized variant (mid-pipeline) and later
// upgrade to the watermarked one; the kind lets the cache enforce upgrade-only.
function bestThumb(w: WorkflowItem): CachedThumb | null {
  if (!w.manifest) return null;
  const m = w.manifest;
  const wmMedium = m.watermarked?.medium;
  const wmLarge = m.watermarked?.large;
  if (wmMedium && wmLarge) {
    return {
      kind: 'watermarked',
      thumb: {
        imageId: m.imageId,
        description: m.description ?? '',
        labels: m.labels ?? [],
        thumbUrl: publicUrl(wmMedium.key),
        largeUrl: publicUrl(wmLarge.key),
      },
    };
  }
  const szMedium = m.sizes?.medium?.s3Ref;
  const szLarge = m.sizes?.large?.s3Ref;
  if (szMedium && szLarge) {
    return {
      kind: 'resized',
      thumb: {
        imageId: m.imageId,
        description: m.description ?? '',
        labels: m.labels ?? [],
        thumbUrl: publicUrl(szMedium.key),
        largeUrl: publicUrl(szLarge.key),
      },
    };
  }
  return null;
}

// Why: latch the best variant seen per workflow — resized first, then upgrade to
// watermarked when it arrives; never regress (poll responses can be stale or
// lack the in-flight manifest entirely, because the backend caps in-flight
// manifest queries at 10 per poll).
const completedCache = ref<Map<string, CachedThumb>>(new Map());

watch(
  () => props.workflows,
  (workflows) => {
    // `completedCache.value` is a reactive Map proxy, so mutating it in place
    // already notifies dependents — no need to reassign a copy.
    const cache = completedCache.value;
    for (const w of workflows) {
      const existing = cache.get(w.workflowId);
      if (existing?.kind === 'watermarked') continue;
      const candidate = bestThumb(w);
      if (!candidate) continue;
      if (!existing || candidate.kind === 'watermarked') {
        cache.set(w.workflowId, candidate);
      }
    }
  },
  { immediate: true },
);

// Prefer a latched watermarked variant (a later poll may no longer report the
// manifest at all), otherwise take this poll's best and fall back to the cache.
function thumbFor(w: WorkflowItem): CompletedThumb | undefined {
  const cached = completedCache.value.get(w.workflowId);
  if (cached?.kind === 'watermarked') return cached.thumb;
  return bestThumb(w)?.thumb ?? cached?.thumb;
}

// Branch on the workflow status first: the thumb cache is only a rendering
// aid, it must never decide whether a workflow is still running.
function toTile(w: WorkflowItem): Tile {
  if (w.status === 'COMPLETED') {
    const image = thumbFor(w);
    return {
      workflowId: w.workflowId,
      status: 'completed',
      image,
      title: image ? image.description || image.imageId : w.imageId,
    };
  }

  if (w.status === 'RUNNING' || w.status === 'CONTINUED_AS_NEW') {
    return {
      workflowId: w.workflowId,
      status: 'running',
      image: thumbFor(w),
      title: `Running: ${w.currentActivity ?? '…'}`,
    };
  }

  // Terminal but unsuccessful: FAILED / TERMINATED / TIMED_OUT / CANCELED.
  return {
    workflowId: w.workflowId,
    status: 'failed',
    title: w.status,
  };
}

const tiles = computed<Tile[]>(() =>
  // Sort by workflowId (deterministic `image-pipeline-<pipelineId>-<imageId>`) for a stable slot per workflow.
  [...props.workflows]
    .map(toTile)
    .sort((a, b) => a.workflowId.localeCompare(b.workflowId)),
);

type CompletedTile = Tile & { image: CompletedThumb };

const completedTiles = computed<CompletedTile[]>(() =>
  tiles.value.filter((t): t is CompletedTile => t.image != null && t.status === 'completed'),
);

const slotCount = computed<number>(
  // Zero means nothing is known yet (no burst size handed over, no poll landed),
  // so fall back to a placeholder grid.
  () => Math.max(props.expectedCount, tiles.value.length) || DEFAULT_PLACEHOLDER_SLOTS,
);

type Slot = { kind: 'tile'; tile: Tile } | { kind: 'pending'; index: number };

const slots = computed<Slot[]>(() =>
  Array.from({ length: slotCount.value }, (_, i): Slot => {
    const tile = tiles.value[i];
    return tile ? { kind: 'tile', tile } : { kind: 'pending', index: i };
  }),
);

// Why a workflow id and not a position: `completedTiles` grows on every poll,
// so a stored index would point at a different image moments later.
const selectedId = ref<string | null>(null);
const closeButton = ref<HTMLButtonElement | null>(null);
const prevButton = ref<HTMLButtonElement | null>(null);
const nextButton = ref<HTMLButtonElement | null>(null);
let previouslyFocused: HTMLElement | null = null;

const selected = computed<CompletedThumb | null>(() => {
  if (selectedId.value === null) return null;
  const tile = completedTiles.value.find((t) => t.workflowId === selectedId.value);
  return tile?.image ?? null;
});

function openModal(tile: CompletedTile): void {
  previouslyFocused
    = typeof document !== 'undefined'
      ? (document.activeElement as HTMLElement | null)
      : null;
  selectedId.value = tile.workflowId;
}

function closeModal(): void {
  selectedId.value = null;
}

// Resolve the current position at call time, against the latest tile order.
function step(delta: number): void {
  const tiles = completedTiles.value;
  if (selectedId.value === null || tiles.length <= 1) return;
  const index = tiles.findIndex((t) => t.workflowId === selectedId.value);
  if (index === -1) return;
  selectedId.value = tiles[(index + delta + tiles.length) % tiles.length]!.workflowId;
}

function prev(): void {
  step(-1);
}

function next(): void {
  step(1);
}

// Cycle Tab/Shift+Tab among the modal's focusable controls. Kept inline
// to avoid a focus-trap dependency; the modal has at most three controls.
function trapFocus(e: KeyboardEvent): void {
  const order = [prevButton.value, nextButton.value, closeButton.value].filter(
    (el): el is HTMLButtonElement => el != null,
  );
  if (order.length === 0) return;
  const active = document.activeElement as HTMLElement | null;
  const idx = active ? order.indexOf(active as HTMLButtonElement) : -1;
  const delta = e.shiftKey ? -1 : 1;
  const nextIdx = (idx + delta + order.length) % order.length;
  e.preventDefault();
  order[nextIdx]!.focus();
}

function onKeydown(e: KeyboardEvent): void {
  if (e.key === 'Escape') closeModal();
  else if (e.key === 'ArrowLeft') prev();
  else if (e.key === 'ArrowRight') next();
  else if (e.key === 'Tab') trapFocus(e);
}

// Lock body scroll + bind Escape only while a modal is open. Guarded for SSG.
// Watching the boolean, not `selected`: arrow-key navigation swaps one image
// for another and must not re-run the focus and listener side effects.
watch(() => selected.value !== null, (isOpen) => {
  if (typeof document === 'undefined') return;
  if (isOpen) {
    document.documentElement.classList.add('overflow-hidden');
    document.addEventListener('keydown', onKeydown);
    // Wait for the modal to render before moving focus into it.
    void nextTick(() => closeButton.value?.focus());
  } else {
    document.documentElement.classList.remove('overflow-hidden');
    document.removeEventListener('keydown', onKeydown);
    previouslyFocused?.focus();
    previouslyFocused = null;
  }
});

onBeforeUnmount(() => {
  if (typeof document === 'undefined') return;
  document.documentElement.classList.remove('overflow-hidden');
  document.removeEventListener('keydown', onKeydown);
});
</script>

<template>
  <section class="card p-4 space-y-3">
    <header class="flex items-baseline justify-between">
      <h2 class="stat-label">Gallery</h2>
      <span class="text-[11px] text-ink-400 font-mono tabular-nums">
        {{ completedTiles.length }} / {{ slotCount }}
      </span>
    </header>

    <div class="grid grid-cols-3 sm:grid-cols-4 gap-2">
      <template
        v-for="slot in slots"
        :key="slot.kind === 'tile' ? slot.tile.workflowId : `pending-${slot.index}`"
      >
        <button
          v-if="slot.kind === 'tile' && slot.tile.status === 'completed' && slot.tile.image"
          type="button"
          class="group block animate-fade-in text-left
            focus-visible:outline-hidden focus-visible:ring-2
            focus-visible:ring-primary/60 focus-visible:ring-offset-2
            focus-visible:ring-offset-bg rounded-md"
          :aria-label="`Open ${slot.tile.title}`"
          @click="openModal(slot.tile as CompletedTile)"
        >
          <div
            class="aspect-square overflow-hidden rounded-md bg-surface-hover
              border border-surface-border transition-all duration-300
              group-hover:border-primary group-hover:ring-2
              group-hover:ring-primary/60 group-hover:shadow-glow
              group-hover:scale-[1.03] relative"
          >
            <img
              :src="slot.tile.image.thumbUrl"
              :alt="slot.tile.title"
              loading="lazy"
              width="150"
              height="150"
              class="h-full w-full object-cover transition-transform duration-300
                group-hover:scale-105"
            >
            <div
              class="absolute inset-0 bg-linear-to-t from-bg/60 to-transparent
                opacity-0 group-hover:opacity-100 transition-opacity duration-300"
              aria-hidden="true"
            />
          </div>
        </button>

        <div
          v-else-if="slot.kind === 'tile' && slot.tile.status === 'running' && slot.tile.image"
          class="animate-fade-in"
        >
          <div
            class="aspect-square overflow-hidden rounded-md bg-surface-hover
              border border-primary/30 relative animate-pulse-glow"
          >
            <img
              :src="slot.tile.image.thumbUrl"
              :alt="slot.tile.title"
              loading="lazy"
              width="150"
              height="150"
              class="h-full w-full object-cover opacity-70"
            >
            <div
              class="absolute inset-0 flex items-center justify-center bg-bg/30"
              aria-hidden="true"
            >
              <div
                class="h-5 w-5 rounded-full border-2 border-primary/30 border-t-primary animate-spin"
              />
            </div>
          </div>
        </div>

        <div
          v-else-if="slot.kind === 'tile' && slot.tile.status === 'running'"
          class="animate-fade-in"
        >
          <div
            class="aspect-square rounded-md bg-linear-to-br from-primary/10
              to-iris/10 border border-primary/30 flex items-center
              justify-center animate-pulse-glow"
          >
            <div
              class="h-5 w-5 rounded-full border-2 border-primary/30 border-t-primary
                animate-spin"
              aria-hidden="true"
            />
          </div>
        </div>

        <div
          v-else-if="slot.kind === 'tile' && slot.tile.status === 'failed'"
          class="animate-fade-in"
        >
          <div
            class="aspect-square rounded-md bg-rose-500/10 border
              border-rose-500/40 flex items-center justify-center text-rose-400
              text-2xl font-bold"
            role="img"
            :aria-label="`Failed: ${slot.tile.title}`"
          >
            <span aria-hidden="true">×</span>
          </div>
        </div>

        <!-- Reserved slot: either no workflow yet, or a completed one whose
             manifest this poll did not return. -->
        <div
          v-else
          aria-hidden="true"
        >
          <div
            class="aspect-square rounded-md bg-linear-to-br from-primary/10
              to-iris/10 border border-primary/30 animate-pulse-glow"
          />
        </div>
      </template>
    </div>

    <Teleport to="body">
      <div
        v-if="selected"
        class="fixed inset-0 z-50 flex items-center justify-center p-4
          bg-bg/80 backdrop-blur-md animate-fade-in"
        role="dialog"
        aria-modal="true"
        :aria-label="selected.description || selected.imageId"
        @click.self="closeModal"
      >
        <div
          class="card-elevated relative w-full max-w-5xl max-h-[90vh]
            overflow-y-auto p-4 sm:p-6"
        >
          <div class="flex justify-center">
            <div class="relative bg-bg/40 rounded-lg">
              <img
                :src="selected.largeUrl"
                :alt="selected.description || selected.imageId"
                class="block max-h-[80vh] max-w-full object-contain rounded-lg"
              >

              <button
                ref="closeButton"
                type="button"
                class="absolute top-2 right-2 z-10 inline-flex h-9 w-9
                  items-center justify-center rounded-full bg-surface/80
                  border-2 border-surface-border text-ink-200
                  hover:border-primary hover:ring-2 hover:ring-primary/60
                  hover:shadow-glow hover:text-primary transition-colors
                  text-2xl leading-none focus-visible:outline-hidden
                  focus-visible:ring-2 focus-visible:ring-primary/60"
                aria-label="Close"
                @click="closeModal"
              >
                ×
              </button>

              <button
                v-if="completedTiles.length > 1"
                ref="prevButton"
                type="button"
                class="absolute left-2 top-1/2 -translate-y-1/2 z-10 inline-flex
                  h-10 w-10 items-center justify-center rounded-full bg-surface/80
                  border-2 border-surface-border text-ink-200
                  hover:border-primary hover:ring-2 hover:ring-primary/60
                  hover:shadow-glow hover:text-primary transition-colors
                  focus-visible:outline-hidden focus-visible:ring-2
                  focus-visible:ring-primary/60"
                aria-label="Previous image"
                @click="prev"
              >
                <svg
                  xmlns="http://www.w3.org/2000/svg"
                  width="24"
                  height="24"
                  viewBox="0 0 24 24"
                  fill="none"
                  stroke="currentColor"
                  stroke-width="2"
                  stroke-linecap="round"
                  stroke-linejoin="round"
                  aria-hidden="true"
                >
                  <path d="M15 18l-6-6 6-6" />
                </svg>
              </button>

              <button
                v-if="completedTiles.length > 1"
                ref="nextButton"
                type="button"
                class="absolute right-2 top-1/2 -translate-y-1/2 z-10 inline-flex
                  h-10 w-10 items-center justify-center rounded-full bg-surface/80
                  border-2 border-surface-border text-ink-200
                  hover:border-primary hover:ring-2 hover:ring-primary/60
                  hover:shadow-glow hover:text-primary transition-colors
                  focus-visible:outline-hidden focus-visible:ring-2
                  focus-visible:ring-primary/60"
                aria-label="Next image"
                @click="next"
              >
                <svg
                  xmlns="http://www.w3.org/2000/svg"
                  width="24"
                  height="24"
                  viewBox="0 0 24 24"
                  fill="none"
                  stroke="currentColor"
                  stroke-width="2"
                  stroke-linecap="round"
                  stroke-linejoin="round"
                  aria-hidden="true"
                >
                  <path d="M9 18l6-6-6-6" />
                </svg>
              </button>
            </div>
          </div>

          <div class="mt-4 space-y-3">
            <p
              v-if="selected.description"
              class="text-sm text-ink-100 leading-relaxed"
            >
              {{ selected.description }}
            </p>
            <p
              v-else
              class="text-sm text-ink-400 italic"
            >
              No description available.
            </p>

            <div
              v-if="selected.labels.length > 0"
              class="flex flex-wrap gap-1.5"
            >
              <span
                v-for="label in selected.labels"
                :key="label"
                class="chip-accent"
              >
                {{ label }}
              </span>
            </div>
          </div>
        </div>
      </div>
    </Teleport>
  </section>
</template>
