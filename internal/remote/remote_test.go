package remote_test

import (
	"errors"
	"net/http/httptest"
	"testing"

	"github.com/google/go-containerregistry/pkg/registry"
	"github.com/google/go-containerregistry/pkg/v1/empty"
	"github.com/google/go-containerregistry/pkg/v1/random"
	"github.com/sHesl/terraform-provider-imagesync/internal/remote"
)

func TestRemote(t *testing.T) {
	reg := httptest.NewServer(registry.New())
	defer reg.Close()

	stubImg, _ := random.Image(10, 1)
	stubImgRef := reg.URL[7:] + "/stub:latest"

	// Write our random image to our registry...
	if err := remote.SyncImage(stubImg, stubImgRef); err != nil {
		t.Fatal(err)
	}

	// ...then read it back out again to assert the sync was successful
	result, err := remote.GetImage(stubImgRef)
	if err != nil {
		t.Fatal(err)
	}
	d1, _ := result.Digest()
	d2, _ := stubImg.Digest()
	if d1.String() != d2.String() {
		t.Fatalf("expected synced image to have same digest across both registries")
	}

	// Write a second tag of that same image...
	secondStubImgRef := reg.URL[7:] + "/stub:1.0"
	if err := remote.SyncImage(stubImg, secondStubImgRef); err != nil {
		t.Fatal(err)
	}

	// ...and then delete the original tag...
	if err := remote.DeleteImage(stubImgRef); err != nil {
		t.Fatal(err)
	}

	// ...asserting the tag deletion was successful...
	result, err = remote.GetImage(stubImgRef)
	if err == nil || !errors.Is(err, remote.ErrImageNotFound) || result != empty.Image {
		t.Fatalf("expected tag to be deleted from registry, even if another image references digest")
	}

	// ...but also that our second tag still persists in the registry.
	result, err = remote.GetImage(secondStubImgRef)
	if err != nil {
		t.Fatal(err)
	}
	if result == empty.Image {
		t.Fatalf("expected second tag to persist in registry even when original tag is deleted")
	}

	// Now we delete this second tag, and assert the image is entirely gone
	if err := remote.DeleteImage(secondStubImgRef); err != nil {
		t.Fatal(err)
	}
	result, err = remote.GetImage(secondStubImgRef)
	if err == nil || !errors.Is(err, remote.ErrImageNotFound) || result != empty.Image {
		t.Fatalf("expected image to be deleted from registry when all tags are deleted")
	}
}
