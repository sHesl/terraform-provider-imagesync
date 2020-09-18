# imagesync Resource

Resource to specify that the image at `source` should be mounted to `destination`, with the given tag.

## Example Usage

```hcl
resource "imagesync" "busybox_1_32" {
  source      = "registry.hub.docker.com/library/busybox:1.32"
  destination = "gcr.io/my-private-registry/busybox:1.32"
}
```

## Argument Reference

* `source` - (Required) Repository reference to the source image that you wish to mirror.
* `destination` - (Required) Repository reference to the source image that you wish to mirror.

## Attribute Reference

* `id` - Repository reference for the mirrored image in the destination, referenced by the image digest, rather than the tag.
* `source_digest` - Digest of the source image; should always match the digest of the destination image.