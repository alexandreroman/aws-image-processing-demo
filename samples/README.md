# Sample images

This directory holds 50 JPG files named `1.jpg` through `50.jpg`.

The photos come from [Picsum](https://picsum.photos) (CC0, by
Unsplash photographers — free to use, no attribution required).

Run `scripts/fetch-samples.sh` to (re)populate the directory.
Then `scripts/upload-samples.sh` syncs them to the demo S3
bucket under the `samples/` prefix; the compose stack does the
same on startup through the `init` service.

These files are the seed source for that bucket, not part of the
web bundle: the frontend never reads them from disk. It picks S3
keys under `samples/` and loads them through `/images/samples/N.jpg`.
A handful of indexes are intentionally blurred to exercise the
pipeline against degraded inputs.
