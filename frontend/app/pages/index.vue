<script setup lang="ts">
const config = useRuntimeConfig();

useHead({
  title: 'AWS Image Processing Demo — Image Processing Burst',
});
</script>

<template>
  <div class="max-w-7xl mx-auto px-4 sm:px-6 lg:px-8 py-10 space-y-8">
    <section class="grid lg:grid-cols-3 gap-6">
      <div class="lg:col-span-1">
        <ControlPanel />
      </div>

      <div class="lg:col-span-2 card p-6 space-y-4">
        <h1 class="text-2xl font-bold text-ink-900">
          Image-processing burst on
          <span class="text-primary">Temporal Cloud</span> +
          <span class="text-accent">AWS</span>
        </h1>
        <p class="text-ink-700 leading-relaxed">
          Trigger a burst of <em>N</em> images. For each one, a Temporal
          workflow runs eight activities — six of them in parallel —
          resizing to three sizes, generating a Claude-powered description,
          watermarking each size, and persisting the manifest in DynamoDB.
          Watch workflows complete live, then share the session URL with
          anyone for the same shared view.
        </p>

        <dl class="grid sm:grid-cols-3 gap-4 pt-2 text-sm">
          <div>
            <dt class="text-ink-500 font-medium">Orchestration</dt>
            <dd class="text-ink-800 font-semibold">Temporal Cloud</dd>
          </div>
          <div>
            <dt class="text-ink-500 font-medium">Compute</dt>
            <dd class="text-ink-800 font-semibold">
              ECS Fargate + Lambda
            </dd>
          </div>
          <div>
            <dt class="text-ink-500 font-medium">AI</dt>
            <dd class="text-ink-800 font-semibold">Claude Haiku 4.5</dd>
          </div>
        </dl>
      </div>
    </section>

    <footer class="text-center text-sm text-ink-400">
      Source on
      <a
        :href="config.public.githubUrl"
        target="_blank"
        rel="noopener noreferrer"
        class="text-primary hover:underline"
      >
        GitHub
      </a>
      · Apache 2.0
    </footer>
  </div>
</template>
