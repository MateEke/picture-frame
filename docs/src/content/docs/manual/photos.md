---
title: Photos
description: Add and manage the photos on the frame, from local uploads or a synced Immich album.
---

The **Images** page is where the frame's photos live. What it shows depends on the photo
library backend: local files you upload, or a synced [Immich](https://immich.app) album. You
pick the backend in **Settings → Photo library** (see [Configuration basics](/getting-started/configuration/)).

## Local photos

With the default **local files** backend, you upload and manage photos here directly.

![The Photos page with the local-files backend: an upload area above the photo grid](../../../assets/screenshots/photos-local.png)

### Uploading

Drag image files onto the upload area, or click it to pick them. Each one opens a quick
cropper so you can frame it for the screen.

![The cropper, framing a photo before upload](../../../assets/screenshots/cropper.png)

Drag to position and zoom to frame the photo, then confirm. The frame stores the cropped
result, so large camera files are trimmed to what the screen actually shows.

### Managing the grid

Uploaded photos appear in the grid below. The one currently on the frame is marked with an
**On screen** badge. Hover a photo for a delete button, or click it to open a larger preview.

To remove several at once, use **Select**, tick the photos you want gone, and **Delete** them
in one step. Both single and bulk deletes ask for confirmation first.

![Select mode: several photos ticked, with a Delete button for the whole batch](../../../assets/screenshots/bulk-delete.png)

## Using Immich instead

To pull photos from [Immich](https://immich.app) rather than uploading them, switch the backend
to **immich** in **Settings → Photo library** and give it a shared-album link:

1. In Immich, create a shared link for the album you want on the frame. A password is optional.
2. Paste the share URL (and password, if any) into the Photo library settings.
3. Set how often the frame reconciles with the album, then save and restart.

The frame then keeps a local copy of the album in sync, and the Images page becomes read-only:

![The Photos page with Immich: a status card above the synced album](../../../assets/screenshots/immich-photos.png)

The status card shows when the album last synced and how many photos it holds. **Sync now**
reconciles immediately instead of waiting for the next interval, **Open album** jumps to the
album in Immich, and a failed sync is flagged here with the reason.

:::note
On Immich 2.6.0 and newer, the frame exchanges the share password for a session token, so it
stays out of request URLs and server logs. Older servers fall back to sending it as a query
parameter on every request. Either way, keep your Immich server behind TLS and avoid reusing a
high-value password for the link.
:::

## Where photos live

Local uploads and the synced Immich cache both sit under the images directory
(`slideshow.images_dir`, default `images`). The backend and Immich link are stored under
`[library]`. Every key is documented in the [configuration reference](/reference/configuration/).
