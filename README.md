# terraform-provider-imagesync
Sync container images across registries, while tracking changes to versions and tags all through Terraform state, allowing you to mirror images in your private registries, alongside the Terraform resources that use them.

```
resource "imagesync" "busybox_1_32" {
  source      = "registry.hub.docker.com/library/busybox:1.32"
  destination = "gcr.io/my-private-registry/busybox:1.32"
}

resource "kubernetes_deployment" "hi_busybox" {
  spec {
    ... 
    template {
      ...
      spec {
        container {
          name  = "hi-busybox"
          image = imagesync.busybox_1_32.id // gcr.io/my-private-registry/busybox@sha256:xxx
        }
      }
    }
  }
}
```

### Supported Operations:
- Syncing images between the `source` and `destination` registries
- Deleting images from the `destination` registry when an `imagesync` resource is removed
- Tracking changes between the underlying tags; if the digest has changed, the `imagesync` will trigger a re-sync

### Supported Registries:
- registry.hub.docker.com (pull public images only)
- quay.io (pull public images only)
- *.gcr.io (using [application default credentials](https://godoc.org/golang.org/x/oauth2/google#FindDefaultCredentials))

Additional registries and/or authentication methods may be added in the future.

## Provider Reference
This provider is hosted in the Terraform registry. Include it in your `main.tf` file in the `terraform` configuration block.
```hcl
terraform {
  required_version = ">= 0.13.1"

  required_providers {
    imagesync = {
      source = "sHesl/imagesync"
      version = "0.0.2"
    }
  }

  backend "gcs" {}
}
```

This provider has only been tested with Terraform 0.13 and above, though it will most likely work without issues for version >0.10.

## Usage Notes

#### Reference images by id, not by destination
It is always preferable to use the digest of an image when specifying which images should run. The `id` of the `imagesync` resource contains the digest, while the `destination` can specify either a tag or a digest. Remember, new versions of an image can overwrite previous versions with the same tag; there is no guarantee you're running the same image you deployed last time if you are just using the tag. Tags are for humans, systems should use digests. 

#### Triggering an image to be sync'd
If a new `imagesync` resource is specified in your state, the creation of that resource will trigger a sync between the `source` and `destination`. If the resource exists in Terraform, but the image at the `destination` was deleted _outside_ of Terraform, your next plan/apply will detect the absence of the image and will run another sync to re-populate the `destination`.

#### Changing versions
If you wish to bump/rollback a version, changing the `source` value will trigger a full tear-down, re-sync cycle, destroying the old image and syncing the new version into the registry. If you wish to keep the old version around for a while, it is recommended to create a separate resource, deleting the old resource when you no longer need the old version around.

#### Retagging the destination
If you wish to change the tag for the destination, this too triggers a full tear-down, re-sync cycle; you will lose the old tag in the registry. If you wish to have multiple tags for a single image, write multiple `imagesync` resources, one for each tag.

#### Deletions
If the plan specifies a resource deletion, either because a change to the source/destination has been specified (triggering a full tear-down and re-sync), or because the resource has been removed, a deletion of this tag will be performed (unless `prevent_destroy` is specified). However, the image layers will only be deleted if no other images in the registry reference these layers. In order for the provider to determine this, it must read every manifest for every image in the repository; this may be a long running operation if you store many tags. 