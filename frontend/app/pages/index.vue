<script setup lang="ts">
useHead({
  title: 'AWS Image Processing Demo — Image Processing Burst',
});

const reasons = [
  {
    title: 'Durable execution',
    body: 'One image = one workflow. Crashes, deploys, scale-downs: Temporal replays from history, no progress is lost.',
    iconKey: 'shield',
    accent: 'text-primary',
  },
  {
    title: 'Elastic compute, zero servers',
    body: 'Fargate workers, scaled by a single lever (min/max-capacity). No nodes, no AMIs, no patching.',
    iconKey: 'bolt',
    accent: 'text-accent',
  },
  {
    title: 'Lifecycle baked into S3',
    body: 'Uploads expire after 7 days, derivatives after 30. Cost is bounded by design, not by a cron.',
    iconKey: 'cycle',
    accent: 'text-iris',
  },
] as const;
</script>

<template>
  <div class="max-w-7xl mx-auto px-4 sm:px-6 lg:px-8 py-6 space-y-5">
    <section class="grid lg:grid-cols-12 gap-4">
      <div
        class="lg:col-span-8 card relative overflow-hidden p-5 sm:p-6
          flex flex-col gap-4"
      >
        <div
          class="absolute inset-0 bg-grid opacity-40 [background-size:24px_24px]
            pointer-events-none"
          aria-hidden="true"
        />

        <div class="relative flex flex-wrap items-center gap-2">
          <span class="chip-primary">Live demo</span>
          <span class="chip">Burst pipeline</span>
          <span class="chip">Durable execution</span>
        </div>

        <h1
          class="relative text-2xl sm:text-3xl font-bold tracking-tight
            text-ink-50 leading-tight"
        >
          Image-processing burst on
          <span
            class="bg-gradient-to-r from-primary via-primary-300 to-iris
              bg-clip-text text-transparent"
          >Temporal Cloud</span>
          +
          <span class="text-accent">AWS</span>
        </h1>

        <p class="relative text-ink-200 text-sm sm:text-[15px] leading-relaxed">
          Trigger a burst of <em class="text-ink-100 not-italic font-medium">N</em>
          images. For each one, a Temporal workflow runs eight activities —
          six of them in parallel — resizing to three sizes, generating a
          Claude-powered description, watermarking each size, and persisting
          the manifest in DynamoDB. Watch workflows complete live, then
          share the pipeline URL for the same live view.
        </p>
      </div>

      <div class="lg:col-span-4">
        <ControlPanel />
      </div>
    </section>

    <LiveCounters />
    <ArchitectureDiagram />

    <section class="grid grid-cols-1 md:grid-cols-3 gap-4">
      <article v-for="r in reasons" :key="r.title" class="card p-5 sm:p-6">
        <div
          class="w-10 h-10 rounded-lg bg-surface-elevated flex items-center
            justify-center mb-3"
          :class="r.accent"
        >
          <svg
            viewBox="0 0 24 24"
            fill="none"
            stroke="currentColor"
            stroke-width="1.75"
            stroke-linecap="round"
            stroke-linejoin="round"
            class="w-5 h-5"
            aria-hidden="true"
          >
            <template v-if="r.iconKey === 'shield'">
              <path d="M12 22s8-4 8-10V5l-8-3-8 3v7c0 6 8 10 8 10z" />
              <path d="M9 12l2 2 4-4" />
            </template>
            <template v-else-if="r.iconKey === 'bolt'">
              <path d="M13 2L4 14h7l-1 8 9-12h-7l1-8z" />
            </template>
            <template v-else-if="r.iconKey === 'cycle'">
              <path d="M21 12a9 9 0 0 1-9 9 9.75 9.75 0 0 1-6.74-2.74L3 16" />
              <path d="M3 22v-6h6" />
              <path d="M3 12a9 9 0 0 1 9-9 9.75 9.75 0 0 1 6.74 2.74L21 8" />
              <path d="M21 2v6h-6" />
            </template>
          </svg>
        </div>
        <h3 class="text-base font-semibold text-ink-100 mb-2">{{ r.title }}</h3>
        <p class="text-sm text-ink-200 leading-relaxed">{{ r.body }}</p>
      </article>
    </section>
  </div>
</template>
