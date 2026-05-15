<script setup lang="ts">
useHead({
  title: 'AWS Image Processing Demo — Image Processing Burst',
});

const techStack = [
  { label: 'Orchestration', value: 'Temporal Cloud', accent: 'primary' },
  { label: 'Compute', value: 'ECS Fargate + Lambda', accent: 'accent' },
  { label: 'Storage', value: 'S3 + DynamoDB', accent: 'accent' },
  { label: 'AI', value: 'Claude Haiku 4.5', accent: 'iris' },
] as const;
</script>

<template>
  <div class="max-w-7xl mx-auto px-4 sm:px-6 lg:px-8 py-6 space-y-5">
    <section class="grid lg:grid-cols-12 gap-4">
      <div
        class="lg:col-span-8 card relative overflow-hidden p-5 sm:p-6
          flex flex-col gap-4"
      >
        <!-- subtle dotted background -->
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

        <dl
          class="relative grid grid-cols-2 sm:grid-cols-4 gap-3 pt-1
            border-t border-surface-border mt-1 pt-3"
        >
          <div v-for="t in techStack" :key="t.label" class="min-w-0">
            <dt class="stat-label">{{ t.label }}</dt>
            <dd
              class="mt-1 text-sm font-semibold text-ink-100 truncate"
              :title="t.value"
            >
              {{ t.value }}
            </dd>
          </div>
        </dl>
      </div>

      <div class="lg:col-span-4">
        <ControlPanel />
      </div>
    </section>

    <LiveCounters />
  </div>
</template>
