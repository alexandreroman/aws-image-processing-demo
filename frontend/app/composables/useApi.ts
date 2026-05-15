// Typed wrapper around `$fetch` that prefixes every call with the configured
// API base + `/api`. All component code must go through `useApi` — no raw
// `$fetch` elsewhere.

export interface S3Ref {
  bucket: string;
  key: string;
}

export interface StartWorkflowsRequest {
  images: S3Ref[];
}

export interface StartWorkflowsResponse {
  pipelineId: string;
  workflowIds: string[];
}

export type WorkflowStatus =
  | 'RUNNING'
  | 'COMPLETED'
  | 'FAILED'
  | 'CANCELED'
  | 'TERMINATED'
  | 'TIMED_OUT'
  | 'CONTINUED_AS_NEW';

export interface ManifestSize {
  s3Ref: S3Ref;
  width: number;
  height: number;
  bytes: number;
}

export interface Manifest {
  pipelineId: string;
  imageId: string;
  original: S3Ref;
  sizes: Record<string, ManifestSize>;
  description?: string;
  labels?: string[];
  watermarked?: Record<string, S3Ref>;
}

export interface WorkflowItem {
  workflowId: string;
  imageId: string;
  status: WorkflowStatus;
  currentActivity?: string;
  startedAt?: string;
  completedAt?: string;
  manifest?: Manifest;
}

export interface PipelineSummary {
  total: number;
  running: number;
  completed: number;
  failed: number;
}

export interface Pipeline {
  pipelineId: string;
  createdAt: string;
  imageCount: number;
  summary: PipelineSummary;
  workflows: WorkflowItem[];
}

type FetchOptions = Parameters<typeof $fetch>[1];

export function useApi() {
  const config = useRuntimeConfig();
  const baseUrl = `${config.public.apiBase.replace(/\/$/, '')}/api`;

  function apiFetch<T>(path: string, opts?: FetchOptions): Promise<T> {
    const url = `${baseUrl}${path.startsWith('/') ? path : `/${path}`}`;
    return $fetch<T>(url, opts);
  }

  function startWorkflows(
    images: S3Ref[],
  ): Promise<StartWorkflowsResponse> {
    return apiFetch<StartWorkflowsResponse>('/workflows/start', {
      method: 'POST',
      body: { images } satisfies StartWorkflowsRequest,
    });
  }

  function getPipeline(pipelineId: string): Promise<Pipeline> {
    return apiFetch<Pipeline>(`/pipelines/${encodeURIComponent(pipelineId)}`);
  }

  return {
    startWorkflows,
    getPipeline,
  };
}
