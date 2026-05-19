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

function publicUrl(bucket: string, key: string): string {
  const base = config.public.s3PublicUrl.replace(/\/$/, '');
  if (!base) {
    // Same-origin via reverse proxy (/images/* → S3 in prod, Moto in compose).
    return `/images/${key}`;
  }
  return `${base}/${bucket}/${key}`;
}

interface CompletedThumb {
  workflowId: string;
  imageId: string;
  description: string;
  labels: string[];
  thumbUrl: string;
  largeUrl: string;
}

type TileStatus = 'completed' | 'running' | 'failed';

// image: when status='completed' the final variant, when status='running' an in-flight resized preview behind the spinner.
interface Tile {
  workflowId: string;
  status: TileStatus;
  currentActivity?: string;
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
        workflowId: w.workflowId,
        imageId: m.imageId,
        description: m.description ?? '',
        labels: m.labels ?? [],
        thumbUrl: publicUrl(wmMedium.bucket, wmMedium.key),
        largeUrl: publicUrl(wmLarge.bucket, wmLarge.key),
      },
    };
  }
  const szMedium = m.sizes?.medium?.s3Ref;
  const szLarge = m.sizes?.large?.s3Ref;
  if (szMedium && szLarge) {
    return {
      kind: 'resized',
      thumb: {
        workflowId: w.workflowId,
        imageId: m.imageId,
        description: m.description ?? '',
        labels: m.labels ?? [],
        thumbUrl: publicUrl(szMedium.bucket, szMedium.key),
        largeUrl: publicUrl(szLarge.bucket, szLarge.key),
      },
    };
  }
  return null;
}

// Why: latch the best variant seen per workflow — resized first, then upgrade to
// watermarked when it arrives; never regress (poll responses can be stale or
// lack the in-flight manifest entirely).
const completedCache = ref<Map<string, CachedThumb>>(new Map());

watchEffect(() => {
  let changed = false;
  const next = completedCache.value;
  for (const w of props.workflows) {
    const existing = next.get(w.workflowId);
    if (existing?.kind === 'watermarked') continue;
    const candidate = bestThumb(w);
    if (!candidate) continue;
    if (!existing) {
      next.set(w.workflowId, candidate);
      changed = true;
    } else if (candidate.kind === 'watermarked') {
      next.set(w.workflowId, candidate);
      changed = true;
    }
  }
  if (changed) {
    completedCache.value = new Map(next);
  }
});

function toTile(w: WorkflowItem): Tile {
  const cached = completedCache.value.get(w.workflowId);
  if (cached) {
    if (cached.kind === 'watermarked') {
      return {
        workflowId: w.workflowId,
        status: 'completed',
        image: cached.thumb,
        title: cached.thumb.description || cached.thumb.imageId,
      };
    }
    return {
      workflowId: w.workflowId,
      status: 'running',
      currentActivity: w.currentActivity,
      image: cached.thumb,
      title: `Running: ${w.currentActivity ?? '…'}`,
    };
  }
  if (
    w.status === 'COMPLETED'
    || w.status === 'RUNNING'
    || w.status === 'CONTINUED_AS_NEW'
  ) {
    const candidate = bestThumb(w);
    if (candidate) {
      if (candidate.kind === 'watermarked') {
        return {
          workflowId: w.workflowId,
          status: 'completed',
          image: candidate.thumb,
          title: candidate.thumb.description || candidate.thumb.imageId,
        };
      }
      return {
        workflowId: w.workflowId,
        status: 'running',
        currentActivity: w.currentActivity,
        image: candidate.thumb,
        title: `Running: ${w.currentActivity ?? '…'}`,
      };
    }
    if (w.status === 'RUNNING' || w.status === 'CONTINUED_AS_NEW') {
      return {
        workflowId: w.workflowId,
        status: 'running',
        currentActivity: w.currentActivity,
        title: `Running: ${w.currentActivity ?? '…'}`,
      };
    }
  }
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

const slotCount = computed<number>(() => {
  if (props.expectedCount === 0 && tiles.value.length === 0) {
    return DEFAULT_PLACEHOLDER_SLOTS;
  }
  return Math.max(props.expectedCount, tiles.value.length);
});

type Slot = { kind: 'tile'; tile: Tile } | { kind: 'pending'; index: number };

const slots = computed<Slot[]>(() =>
  Array.from({ length: slotCount.value }, (_, i): Slot => {
    const tile = tiles.value[i];
    return tile ? { kind: 'tile', tile } : { kind: 'pending', index: i };
  }),
);

const selectedIndex = ref<number | null>(null);
const closeButton = ref<HTMLButtonElement | null>(null);
const prevButton = ref<HTMLButtonElement | null>(null);
const nextButton = ref<HTMLButtonElement | null>(null);
let previouslyFocused: HTMLElement | null = null;

const selected = computed<CompletedThumb | null>(() => {
  if (selectedIndex.value === null) return null;
  return completedTiles.value[selectedIndex.value]?.image ?? null;
});

function openModal(tile: CompletedTile): void {
  const index = completedTiles.value.findIndex(
    (t) => t.workflowId === tile.workflowId,
  );
  if (index === -1) return;
  previouslyFocused
    = typeof document !== 'undefined'
      ? (document.activeElement as HTMLElement | null)
      : null;
  selectedIndex.value = index;
}

function closeModal(): void {
  selectedIndex.value = null;
}

function prev(): void {
  if (selectedIndex.value === null || completedTiles.value.length <= 1) return;
  const n = completedTiles.value.length;
  selectedIndex.value = (selectedIndex.value - 1 + n) % n;
}

function next(): void {
  if (selectedIndex.value === null || completedTiles.value.length <= 1) return;
  selectedIndex.value = (selectedIndex.value + 1) % completedTiles.value.length;
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
watch(selected, (item) => {
  if (typeof document === 'undefined') return;
  if (item) {
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
            focus-visible:outline-none focus-visible:ring-2
            focus-visible:ring-primary/60 focus-visible:ring-offset-2
            focus-visible:ring-offset-bg rounded-md"
          :title="slot.tile.title"
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
              class="absolute inset-0 bg-gradient-to-t from-bg/60 to-transparent
                opacity-0 group-hover:opacity-100 transition-opacity duration-300"
              aria-hidden="true"
            />
          </div>
        </button>

        <div
          v-else-if="slot.kind === 'tile' && slot.tile.status === 'running' && slot.tile.image"
          class="animate-fade-in"
          :title="slot.tile.title"
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
          :title="slot.tile.title"
        >
          <div
            class="aspect-square rounded-md bg-gradient-to-br from-primary/10
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
          v-else-if="slot.kind === 'tile'"
          class="animate-fade-in"
          :title="slot.tile.title"
        >
          <div
            class="aspect-square rounded-md bg-rose-500/10 border
              border-rose-500/40 flex items-center justify-center text-rose-400
              text-2xl font-bold"
            aria-hidden="true"
          >
            ×
          </div>
        </div>

        <div
          v-else
          aria-hidden="true"
          title="Waiting for workflow..."
        >
          <div
            class="aspect-square rounded-md bg-gradient-to-br from-primary/10
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
                  text-2xl leading-none focus-visible:outline-none
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
                  focus-visible:outline-none focus-visible:ring-2
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
                  focus-visible:outline-none focus-visible:ring-2
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
