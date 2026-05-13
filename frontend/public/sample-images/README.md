# Sample images

This directory holds 50 JPG files named `1.jpg` through `50.jpg`.

The photos come from [Picsum](https://picsum.photos) (CC0, by
Unsplash photographers — free to use, no attribution required).

Run `scripts/fetch-samples.sh` to (re)populate the directory.
Then `scripts/upload-samples.sh` syncs them to the demo S3
bucket under the `samples/` prefix.

The frontend picks from these at random when the user clicks
**Start burst** on the landing page. A handful of indexes are
intentionally blurred to exercise the pipeline against degraded
inputs.
