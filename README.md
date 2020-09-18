# terraform-provider-imagesync
Sync container images across registries, while tracking changes to versions and tags all through Terraform state, allowing organisations to maintain the images in their private registries alongside the Terraform resources that use them.

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

### Supported Registries:
- registry.hub.docker.com (pull public images only)
- quay.io (pull public images only)
- *.gcr.io (using [application default credentials](https://godoc.org/golang.org/x/oauth2/google#FindDefaultCredentials))

Additional registries and/or authentication methods may be added in the future.
