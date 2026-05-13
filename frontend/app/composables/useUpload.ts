// Upload files directly to S3 via presigned PUT URLs. The backend hands
// back one URL per file; bytes never touch it.

import type { S3Ref } from './useApi';

export interface UploadProgress {
  file: File;
  key: string;
  loaded: number;
  total: number;
  status: 'pending' | 'uploading' | 'done' | 'error';
  error?: string;
}

export interface UseUploadReturn {
  progress: Ref<UploadProgress[]>;
  uploading: Ref<boolean>;
  upload: (files: File[]) => Promise<S3Ref[]>;
  reset: () => void;
}

export function useUpload(): UseUploadReturn {
  const api = useApi();

  const progress = ref<UploadProgress[]>([]);
  const uploading = ref(false);

  function reset() {
    progress.value = [];
  }

  // Extract the bucket name from a presigned URL.
  // Handles both virtual-hosted ("https://{bucket}.s3.amazonaws.com/...")
  // and path-style ("https://localhost:4566/{bucket}/...") forms used by
  // LocalStack.
  function bucketFromPresignedUrl(rawUrl: string, key: string): string {
    const parsed = new URL(rawUrl);
    const hostParts = parsed.hostname.split('.');
    if (hostParts.length > 0 && hostParts[1] === 's3') {
      return hostParts[0]!;
    }
    // Path-style: /{bucket}/{key...}
    const path = parsed.pathname.replace(/^\/+/, '');
    if (path.endsWith(key)) {
      return path.slice(0, path.length - key.length).replace(/\/+$/, '');
    }
    return path.split('/')[0] ?? '';
  }

  function putWithProgress(
    url: string,
    file: File,
    onProgress: (loaded: number, total: number) => void,
  ): Promise<void> {
    return new Promise((resolve, reject) => {
      const xhr = new XMLHttpRequest();
      xhr.open('PUT', url);
      xhr.setRequestHeader(
        'Content-Type',
        file.type || 'application/octet-stream',
      );
      xhr.upload.onprogress = (e) => {
        if (e.lengthComputable) {
          onProgress(e.loaded, e.total);
        }
      };
      xhr.onload = () => {
        if (xhr.status >= 200 && xhr.status < 300) {
          resolve();
        } else {
          reject(new Error(`S3 PUT failed: HTTP ${xhr.status}`));
        }
      };
      xhr.onerror = () => reject(new Error('S3 PUT network error'));
      xhr.send(file);
    });
  }

  async function upload(files: File[]): Promise<S3Ref[]> {
    if (files.length === 0) {
      return [];
    }

    uploading.value = true;
    try {
      const presigned = await api.presignUploads(files.length);
      if (presigned.length !== files.length) {
        throw new Error(
          `presign returned ${presigned.length} URLs, expected ${files.length}`,
        );
      }

      progress.value = files.map((file, i) => ({
        file,
        key: presigned[i]!.key,
        loaded: 0,
        total: file.size,
        status: 'pending',
      }));

      const refs: S3Ref[] = await Promise.all(
        files.map(async (file, i) => {
          const p = presigned[i]!;
          const slot = progress.value[i]!;
          slot.status = 'uploading';
          try {
            await putWithProgress(p.url, file, (loaded, total) => {
              slot.loaded = loaded;
              slot.total = total;
            });
            slot.status = 'done';
            return { bucket: bucketFromPresignedUrl(p.url, p.key), key: p.key };
          } catch (err) {
            slot.status = 'error';
            slot.error = err instanceof Error ? err.message : String(err);
            throw err;
          }
        }),
      );

      return refs;
    } finally {
      uploading.value = false;
    }
  }

  return { progress, uploading, upload, reset };
}
