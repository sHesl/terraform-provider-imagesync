package imagesync

import (
	"fmt"
	"net"
	"net/http"
	"strings"

	"github.com/google/go-containerregistry/pkg/authn"
	"github.com/google/go-containerregistry/pkg/name"
	"github.com/google/go-containerregistry/pkg/v1/google"
	"github.com/google/go-containerregistry/pkg/v1/remote"
	"github.com/google/go-containerregistry/pkg/v1/remote/transport"
	"github.com/hashicorp/terraform/helper/schema"
)

func imagesync() *schema.Resource {
	return &schema.Resource{
		Create: imagesyncCreate,
		Update: imagesyncUpdate,
		Read:   imagesyncRead,
		Delete: imagesyncDelete,
		Importer: &schema.ResourceImporter{
			State: schema.ImportStatePassthrough,
		},

		SchemaVersion: 1,

		Schema: map[string]*schema.Schema{
			"source": {
				Type:     schema.TypeString,
				Required: true,
			},
			"destination": {
				Type:     schema.TypeString,
				Required: true,
				ForceNew: true,
			},
			"source_digest": {
				Type:     schema.TypeString,
				Computed: true,
				ForceNew: true,
			},
		},

		CustomizeDiff: sourceChangedDiffFunc,
	}
}

func imagesyncCreate(d *schema.ResourceData, m interface{}) error {
	src := d.Get("source").(string)
	srcImg, exists, err := getRemoteImage(src)
	if err != nil {
		return err
	}
	if !exists {
		return fmt.Errorf("unable to locate source image at '%s'", src)
	}

	dest := d.Get("destination").(string)
	destRef, err := name.ParseReference(dest, name.WeakValidation)
	if err != nil {
		return err
	}

	destAuthOpt, err := authOption(destRef)
	if err != nil {
		return err
	}

	if err := remote.Write(destRef, srcImg, destAuthOpt); err != nil {
		return err
	}

	return imagesyncRead(d, m) // Resync the state to ensure the digest and ID of state match remote img
}

func imagesyncUpdate(d *schema.ResourceData, m interface{}) error {
	// Updates can only be triggered by 'source' changes that *don't* change the 'source_digest', suggesting a
	// new registry/tag, but not a new underlying image. No actual update is necessary.
	return imagesyncRead(d, m)
}

func imagesyncRead(d *schema.ResourceData, meta interface{}) error {
	dest := d.Get("destination").(string)
	destImg, exists, err := getRemoteImage(dest)
	if err != nil {
		return err
	}

	if !exists {
		d.SetId("")
	} else {
		imgID, err := imageID(dest, destImg)
		if err != nil {
			return err
		}
		d.SetId(imgID)
	}

	return nil
}

func imagesyncDelete(d *schema.ResourceData, m interface{}) error {
	dest := d.Get("destination").(string)
	destRef, err := name.ParseReference(dest, name.WeakValidation)
	if err != nil {
		return err
	}

	destAuthOpt, err := authOption(destRef)
	if err != nil {
		return err
	}

	// Delete this tag. Perform this regardless of if other tags exist
	if err := remote.Delete(destRef, destAuthOpt); err != nil {
		return err
	}

	// Check through all available tags to see if there are any more images referencing these blobs
	tags, err := remote.List(destRef.Context(), destAuthOpt)
	if err != nil {
		if strings.Contains(err.Error(), "METHOD_UNKNOWN") {
			// If the registry doesn't support listing images, we can't be sure we can safely delete these blobs
			return nil
		}
		return err
	}

	for _, t := range tags {
		imgRef, err := name.ParseReference(destRef.Context().String()+":"+t, name.WeakValidation)
		if err != nil {
			return err
		}

		i, err := remote.Image(imgRef, destAuthOpt)
		if err != nil {
			return err
		}

		imageID, err := i.Digest()
		if err != nil {
			return err
		}

		if imageID.String() == digestFromReference(d.Id()) {
			return nil // Another image is using the same layers as we are, do not delete these layers!
		}
	}

	// No other tag references these layers, we're free to delete
	idRef, err := name.ParseReference(d.Id(), name.WeakValidation)
	if err != nil {
		return err
	}

	return remote.Delete(idRef, destAuthOpt)
}

func sourceChangedDiffFunc(d *schema.ResourceDiff, v interface{}) error {
	// Several things could have changed with the 'source', it could be that:
	// - the user wants to use a different image
	// - the source image in the registry has changed
	// - the user wants the same image, but from a different registry
	// If the first 2 are true, the digest will change, and so 'ForceNew' will be triggered,
	// If the image digest remains the same, then the resource will not be marked for update
	src := d.Get("source").(string)
	srcImg, exists, err := getRemoteImage(src)
	if err != nil {
		return err
	}
	if !exists {
		return fmt.Errorf("unable to locate source image at '%s'", src)
	}
	srcDigest, err := srcImg.Digest()
	if err != nil {
		return err
	}

	oldDigest := d.Get("source_digest").(string)
	newDigest := srcDigest.String()
	if oldDigest != newDigest {
		d.SetNew("source_digest", newDigest)
	}

	return nil
}

func authOption(ref name.Reference) (remote.Option, error) {
	reg := ref.Context().Registry

	switch {
	case registryIn(reg, "registry.hub.docker.com", "quay.io", "ghcr.io"):
		scopes := []string{ref.Scope(transport.PullScope)}
		rt, err := transport.New(reg, &authn.Bearer{}, http.DefaultTransport, scopes)
		if err != nil {
			return nil, fmt.Errorf("resolve repository: %w", err)
		}
		return remote.WithTransport(rt), err
	case registryIn(reg, "gcr.io", "eu.gcr.io", "us.gcr.io", "asia.gcr.io"):
		googleAuth, err := google.NewEnvAuthenticator()
		if err != nil {
			return nil, err
		}
		return remote.WithAuth(googleAuth), nil
	default:
		return remote.WithAuth(authn.Anonymous), nil
	}
}

func registryIn(r name.Registry, in ...string) bool {
	for _, s := range in {
		if r.Name() == s {
			return true
		}
	}

	return false
}

func ipFromRegistry(reg string) net.IP {
	if i := strings.Index(reg, ":"); i != -1 && i < len(reg) {
		reg = reg[:i]
	}

	return net.ParseIP(reg)
}
