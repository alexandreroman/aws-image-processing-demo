# Embedded assets

Binary assets compiled into the worker via `go:embed`.

## `temporal-logo.png`

Temporal "lockup horizontal, light" logo — white logo on a transparent
background, designed to sit on dark surfaces. The same artwork is used by
the frontend (`frontend/app/app.vue`).

Source:
<https://images.ctfassets.net/0uuz8ydxyd9p/2ctnUPEhKA75tYnrl2Kzvj/90563965bc4ea2af9442b6eb4ba43180/Temporal_LogoLockup_Horizontal_light_1_2x.png>

The PNG is embedded because the watermark activity runs on Fargate workers
with no guarantee of arbitrary network egress at runtime; downloading it
once at build time keeps the activity self-contained and deterministic.
