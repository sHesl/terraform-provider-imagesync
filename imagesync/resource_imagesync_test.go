package imagesync_test

import (
	"fmt"
	"net/http/httptest"
	"testing"

	"github.com/google/go-containerregistry/pkg/name"
	"github.com/google/go-containerregistry/pkg/registry" // Modified to allow registry deletes
	v1 "github.com/google/go-containerregistry/pkg/v1"
	"github.com/google/go-containerregistry/pkg/v1/random"
	"github.com/google/go-containerregistry/pkg/v1/remote"
	"github.com/hashicorp/terraform/helper/resource"
	"github.com/hashicorp/terraform/terraform"
	"github.com/sHesl/terraform-provider-imagesync/imagesync"
)

func TestImageSync(t *testing.T) {
	srcReg := httptest.NewServer(registry.New())
	defer srcReg.Close()

	destReg := httptest.NewServer(registry.New())
	defer destReg.Close()

	fakeImg, _ := random.Image(10, 1)
	fakeImgDigest, _ := fakeImg.Digest()
	initSrcImage(srcReg, "library/busybox:1.0", fakeImg)
	initSrcImage(srcReg, "library/busybox:latest", fakeImg)

	stubImageSyncConfig := func(srcReg, destReg *httptest.Server, srcTag, destTag string) string {
		return fmt.Sprintf(`resource "imagesync" "unit_test" {
			source      = "%s/library/busybox:%s"
			destination = "%s/busybox:%s"
		}`, srcReg.URL[7:], srcTag, destReg.URL[7:], destTag) // trim http://
	}

	stubImageSyncDockerhubConfig := func(destReg *httptest.Server) string {
		return fmt.Sprintf(`resource "imagesync" "unit_test" {
			source      = "registry.hub.docker.com/library/busybox:1.32"
			destination = "%s/busybox:1.32"
		}`, destReg.URL[7:])
	}

	resource.Test(t, resource.TestCase{
		IsUnitTest:   true,
		PreCheck:     nil,
		Providers:    map[string]terraform.ResourceProvider{"imagesync": imagesync.Provider()},
		CheckDestroy: nil,
		Steps: []resource.TestStep{
			{
				// Create the resource, mirror the image to the dest registry, and correctly set the id (w/digest)
				Config:       stubImageSyncConfig(srcReg, destReg, "1.0", "1.0"),
				ResourceName: "imagesync.unit_test",
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr("imagesync.unit_test", "id", destReg.URL[7:]+"/busybox@"+fakeImgDigest.String()),
					resource.TestCheckResourceAttr("imagesync.unit_test", "source_digest", fakeImgDigest.String()),
					resource.TestCheckResourceAttr("imagesync.unit_test", "source", srcReg.URL[7:]+"/library/busybox:1.0"),
				),
			},

			{
				// Test that updating the source, but with the same digest, does not trigger an update
				Config:       stubImageSyncConfig(srcReg, destReg, "latest", "1.0"),
				ResourceName: "imagesync.unit_test",
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr("imagesync.unit_test", "id", destReg.URL[7:]+"/busybox@"+fakeImgDigest.String()),
					resource.TestCheckResourceAttr("imagesync.unit_test", "source_digest", fakeImgDigest.String()),
					resource.TestCheckResourceAttr("imagesync.unit_test", "source", srcReg.URL[7:]+"/library/busybox:latest"),
				),
			},

			{
				// Test we can pull public dockerhub images
				Config:       stubImageSyncDockerhubConfig(destReg),
				ResourceName: "imagesync.unit_test",
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr("imagesync.unit_test", "id", destReg.URL[7:]+"/busybox@sha256:0415f56ccc05526f2af5a7ae8654baec97d4a614f24736e8eef41a4591f08019"),
					resource.TestCheckResourceAttr("imagesync.unit_test", "source_digest", "sha256:0415f56ccc05526f2af5a7ae8654baec97d4a614f24736e8eef41a4591f08019"),
					resource.TestCheckResourceAttr("imagesync.unit_test", "source", "registry.hub.docker.com/library/busybox:1.32"),
				),
			},
		},
	})
}

func initSrcImage(fakeReg *httptest.Server, path string, img v1.Image) {
	ref, err := name.ParseReference(fakeReg.URL[7:]+"/"+path, name.WeakValidation)
	if err != nil {
		panic(err)
	}

	if err := remote.Write(ref, img); err != nil {
		panic(err)
	}
}
