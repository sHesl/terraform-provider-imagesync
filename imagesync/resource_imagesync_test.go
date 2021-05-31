package imagesync_test

import (
	"errors"
	"fmt"
	"log"
	"net/http"
	"net/http/httptest"
	"regexp"
	"strconv"
	"testing"

	"github.com/sHesl/terraform-provider-imagesync/imagesync"
	"github.com/sHesl/terraform-provider-imagesync/internal/remote"

	"github.com/google/go-containerregistry/pkg/registry"
	v1 "github.com/google/go-containerregistry/pkg/v1"
	"github.com/google/go-containerregistry/pkg/v1/random"
	"github.com/hashicorp/terraform-plugin-sdk/helper/resource"
	"github.com/hashicorp/terraform-plugin-sdk/terraform"
)

func TestImageSync(t *testing.T) {
	srcReg := httptest.NewServer(registry.New(registry.Logger(log.Default())))
	defer srcReg.Close()

	destReg := httptest.NewServer(registry.New(registry.Logger(log.Default())))
	defer destReg.Close()

	fakeImg, _ := random.Image(10, 1)
	fid, _ := fakeImg.Digest()
	digest := fid.String()

	mustInitSrcImage(srcReg, "library/blah@"+digest, fakeImg)

	resWithTag := func(destTag string) string {
		return fmt.Sprintf(`resource "imagesync" "ut_%s" {
			source_digest   = "%s/library/blah@%s"
			destination_tag = "%s/blah:%s"
		}`, destTag, srcReg.URL[7:], digest, destReg.URL[7:], destTag)
	}

	expSrcDigest := srcReg.URL[7:] + "/library/blah@" + digest
	expTagV1 := destReg.URL[7:] + "/blah:v1"
	expTagLatest := destReg.URL[7:] + "/blah:latest"
	expID := destReg.URL[7:] + "/blah@" + digest

	resource.Test(t, resource.TestCase{
		IsUnitTest: true,
		PreCheck:   nil,
		Providers:  map[string]terraform.ResourceProvider{"imagesync": imagesync.Provider()},
		Steps: []resource.TestStep{
			{
				// Create the resource, mirror the image to the dest registry, and correctly set the id (w/digest)
				Config:       resWithTag("v1"),
				ResourceName: "imagesync.ut_v1",
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr("imagesync.ut_v1", "source_digest", expSrcDigest),
					resource.TestCheckResourceAttr("imagesync.ut_v1", "destination_tag", expTagV1),
					resource.TestCheckResourceAttr("imagesync.ut_v1", "id", expID),

					// Check in our in-memory registry that the image actually exists (both by tag and digest)
					assertImageExistsInRemote(expID),
					assertImageExistsInRemote(expTagV1),
				),
			},

			{
				// Re-read the state of our resource, without any changes
				Config:       resWithTag("v1"),
				ResourceName: "imagesync.ut_v1",
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr("imagesync.ut_v1", "source_digest", expSrcDigest),
					resource.TestCheckResourceAttr("imagesync.ut_v1", "destination_tag", expTagV1),
					resource.TestCheckResourceAttr("imagesync.ut_v1", "id", expID),

					// Check in our in-memory registry that the image actually exists (both by tag and digest)
					assertImageExistsInRemote(expID),
					assertImageExistsInRemote(expTagV1),
				),
			},

			{
				// Create a second resource that references the same image, but under a new tag in the destination
				Config: resWithTag("v1") + "\n" + resWithTag("latest"),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr("imagesync.ut_v1", "source_digest", expSrcDigest),
					resource.TestCheckResourceAttr("imagesync.ut_v1", "destination_tag", expTagV1),
					resource.TestCheckResourceAttr("imagesync.ut_v1", "id", expID),

					resource.TestCheckResourceAttr("imagesync.ut_latest", "source_digest", expSrcDigest),
					resource.TestCheckResourceAttr("imagesync.ut_latest", "destination_tag", expTagLatest),
					resource.TestCheckResourceAttr("imagesync.ut_latest", "id", expID),

					// Check in our in-memory registry that a second tag now exists in our remote
					func(*terraform.State) error {
						tags, err := remote.ListTags(destReg.URL[7:] + "/blah")
						if len(tags) != 2 {
							return err
						}
						return nil
					},
				),
			},

			{
				// Delete our first resource (by only specifying resource 2 in our config). The second resource
				// should still be present
				Config:       resWithTag("latest"),
				ResourceName: "imagesync.ut_latest",
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr("imagesync.ut_latest", "source_digest", expSrcDigest),
					resource.TestCheckResourceAttr("imagesync.ut_latest", "destination_tag", expTagLatest),
					resource.TestCheckResourceAttr("imagesync.ut_latest", "id", expID),
					assertImageExistsInRemote(expID),
					assertImageExistsInRemote(expTagLatest),
					assertImageDoesNotExistInRemote(expTagV1),
				),
			},

			{
				// Delete our second tag
				Config:       resWithTag("latest"),
				Destroy:      true,
				ResourceName: "imagesync.ut_latest",
				Check: resource.ComposeTestCheckFunc(
					assertImageDoesNotExistInRemote(expID),
					assertImageDoesNotExistInRemote(expTagLatest),
					assertImageDoesNotExistInRemote(expTagV1),
				),
			},

			// For our final test, we recreate both resources at once, and then delete them simultaneously to prove
			// no race conditions exists when creating/deleting multiple resources.
			{
				Config: resWithTag("v1") + "\n" + resWithTag("latest"),
			},
			{
				Config:  resWithTag("v1") + "\n" + resWithTag("latest"),
				Destroy: true,
			},
		},
		CheckDestroy: resource.ComposeTestCheckFunc(
			assertImageDoesNotExistInRemote(expID),
			assertImageDoesNotExistInRemote(expTagLatest),
			assertImageDoesNotExistInRemote(expTagV1),
		),
	})
}

func TestImageSyncDockerhub(t *testing.T) {
	destReg := httptest.NewServer(registry.New(registry.Logger(log.Default())))
	defer destReg.Close()

	srcDigest := "sha256:ae39a6f5c07297d7ab64dbd4f82c77c874cc6a94cea29fdec309d0992574b4f7"
	expSrc := "registry.hub.docker.com/library/busybox@" + srcDigest

	stubImageSyncDockerhubConfig := func(destReg *httptest.Server) string {
		return fmt.Sprintf(`resource "imagesync" "docker_ut" {
						source_digest   = "%s" 
						destination_tag = "%s/busybox:1.32"
					}`, expSrc, destReg.URL[7:])
	}

	resource.Test(t, resource.TestCase{
		IsUnitTest: true,
		Providers:  map[string]terraform.ResourceProvider{"imagesync": imagesync.Provider()},
		Steps: []resource.TestStep{
			{
				Config:       stubImageSyncDockerhubConfig(destReg),
				ResourceName: "imagesync.docker_ut",
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr("imagesync.docker_ut", "source_digest", expSrc),
					resource.TestCheckResourceAttrSet("imagesync.docker_ut", "destination_tag"),
					resource.TestCheckResourceAttrSet("imagesync.docker_ut", "id"),
					assertImageExistsInRemote(destReg.URL[7:]+"/busybox:1.32"),
				),
			},
		},
	})
}

func TestImageSyncQualys(t *testing.T) {
	destReg := httptest.NewServer(registry.New(registry.Logger(log.Default())))
	defer destReg.Close()

	srcDigest := "sha256:d1cce64093d4a850d28726ec3e48403124808f76567b5bd7b26e4416300996a7"
	expSrc := "quay.io/coreos/prometheus-config-reloader@" + srcDigest

	stubImageSyncQuayConfig := func(destReg *httptest.Server) string {
		return fmt.Sprintf(`resource "imagesync" "quay_ut" {
						source_digest   = "%s"
						destination_tag = "%s/prometheus-config-reloader:v0.38.1"
					}`, expSrc, destReg.URL[7:])
	}

	resource.Test(t, resource.TestCase{
		IsUnitTest: true,
		Providers:  map[string]terraform.ResourceProvider{"imagesync": imagesync.Provider()},
		Steps: []resource.TestStep{
			{
				Config:       stubImageSyncQuayConfig(destReg),
				ResourceName: "imagesync.quay_ut",
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr("imagesync.quay_ut", "source_digest", expSrc),
					resource.TestCheckResourceAttrSet("imagesync.quay_ut", "destination_tag"),
					resource.TestCheckResourceAttrSet("imagesync.quay_ut", "id"),
					assertImageExistsInRemote(destReg.URL[7:]+"/prometheus-config-reloader:v0.38.1"),
				),
			},
		},
	})
}

func TestImageSyncElastic(t *testing.T) {
	destReg := httptest.NewServer(registry.New(registry.Logger(log.Default())))
	defer destReg.Close()

	srcDigest := "sha256:6d54f297c1c17f0da99d1262f6456776f5a47cf0104f1d020679e8273fd51378"
	expSrc := "docker.elastic.co/eck/eck-operator@" + srcDigest

	stubImageSyncElasticConfig := func(destReg *httptest.Server) string {
		return fmt.Sprintf(`resource "imagesync" "elastic_ut" {
			source_digest   = "%s"
			destination_tag = "%s/eck-operator:v1.1"
		}`, expSrc, destReg.URL[7:])
	}

	resource.Test(t, resource.TestCase{
		IsUnitTest: true,
		Providers:  map[string]terraform.ResourceProvider{"imagesync": imagesync.Provider()},
		Steps: []resource.TestStep{
			{
				Config:       stubImageSyncElasticConfig(destReg),
				ResourceName: "imagesync.elastic_ut",
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr("imagesync.elastic_ut", "source_digest", expSrc),
					resource.TestCheckResourceAttrSet("imagesync.elastic_ut", "destination_tag"),
					resource.TestCheckResourceAttrSet("imagesync.elastic_ut", "id"),
					assertImageExistsInRemote(destReg.URL[7:]+"/eck-operator:v1.1"),
				),
			},
		},
	})
}

func TestImageSyncCreateImageNotFound(t *testing.T) {
	srcReg := httptest.NewServer(registry.New(registry.Logger(log.Default())))
	defer srcReg.Close()

	destReg := httptest.NewServer(registry.New(registry.Logger(log.Default())))
	defer destReg.Close()

	fakeImg, _ := random.Image(10, 1)
	mustInitSrcImage(srcReg, "v2/blah:v1", fakeImg)
	mustInitSrcImage(srcReg, "v2/blah:latest", fakeImg)

	aas := "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
	badReference := fmt.Sprintf(`resource "imagesync" "ut" {
			source_digest   = "%s/v2/blah@sha256:%s"
			destination_tag = "%s/blah:bad_tag"
		}`, srcReg.URL[7:], aas, destReg.URL[7:])

	exp := fmt.Sprintf("(127.0.0.1:)(\\d+)(/v2/blah@sha256:%s)' not present in remote. image not found", aas)

	resource.Test(t, resource.TestCase{
		IsUnitTest: true,
		PreCheck:   nil,
		Providers:  map[string]terraform.ResourceProvider{"imagesync": imagesync.Provider()},
		Steps: []resource.TestStep{
			{
				// Test that we gracefully error if the source image does not exist
				Config:       badReference,
				ResourceName: "imagesync.ut",
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr("imagesync.ut", "id", ""),
					resource.TestCheckResourceAttr("imagesync.ut", "source_digest", ""),
					resource.TestCheckResourceAttr("imagesync.ut", "source_digest", srcReg.URL+"/v2/blah:bad_tag"),
				),
				ExpectError: regexp.MustCompile(exp),
			},
		},
	})
}

func TestImageSyncRegistryDriftBetweenSteps(t *testing.T) {
	calls := 0
	srcReg := httptest.NewServer(http.HandlerFunc(func(rw http.ResponseWriter, r *http.Request) {
		rw.Write([]byte(strconv.Itoa(calls)))
		calls++
	}))
	defer srcReg.Close()

	destReg := httptest.NewServer(registry.New(registry.Logger(log.Default())))
	defer destReg.Close()

	fakeImg, _ := random.Image(10, 1)
	mustInitSrcImage(srcReg, "library/blah:v1", fakeImg)

	aas := "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"

	res := func(srcReg, destReg *httptest.Server) string {
		return fmt.Sprintf(`resource "imagesync" "ut" {
			source_digest   = "%s/library/blah@sha256:%s"
			destination_tag = "%s/blah:v1"
		}`, srcReg.URL[7:], aas, destReg.URL[7:])
	}

	resource.Test(t, resource.TestCase{
		PreCheck:  nil,
		Providers: map[string]terraform.ResourceProvider{"imagesync": imagesync.Provider()},
		Steps: []resource.TestStep{
			{
				Config:       res(srcReg, destReg),
				ResourceName: "imagesync.ut",
				ExpectError:  regexp.MustCompile("errors during apply: manifest digest:"),
			},
		},
	})
}

func mustInitSrcImage(fakeReg *httptest.Server, path string, img v1.Image) {
	if err := remote.SyncImage(img, fakeReg.URL[7:]+"/"+path); err != nil {
		panic(err)
	}
}

func assertImageExistsInRemote(ref string) func(*terraform.State) error {
	return func(*terraform.State) error {
		result, err := remote.GetImage(ref)
		if err != nil {
			return fmt.Errorf("unable to read image from remote: %v", err)
		}
		if result == nil {
			return fmt.Errorf("image was not present in remote")
		}

		return nil
	}
}

func assertImageDoesNotExistInRemote(ref string) func(*terraform.State) error {
	return func(*terraform.State) error {
		_, err := remote.GetImage(ref)
		if !errors.Is(err, remote.ErrImageNotFound) {
			return fmt.Errorf("expected image '%s' not to be present in remote", ref)
		}
		return nil
	}
}
