package imagesync

import (
	"errors"
	"fmt"

	"github.com/sHesl/terraform-provider-imagesync/internal/ref"
	"github.com/sHesl/terraform-provider-imagesync/internal/remote"

	"github.com/hashicorp/terraform-plugin-sdk/helper/schema"
)

func imagesync() *schema.Resource {
	return &schema.Resource{
		Create: imagesyncCreate,
		Read:   imagesyncRead,
		Delete: imagesyncDelete,
		Importer: &schema.ResourceImporter{
			State: schema.ImportStatePassthrough,
		},

		SchemaVersion: 1,

		Schema: map[string]*schema.Schema{
			"source_digest": {
				Type:         schema.TypeString,
				Required:     true,
				ForceNew:     true,
				ValidateFunc: validateDigest,
				Description:  "Fully qualified reference to the source image, including the expected digest",
			},
			"destination_tag": {
				Type:         schema.TypeString,
				Required:     true,
				ForceNew:     true,
				ValidateFunc: validateReference,
				Description:  "Fully qualified reference to the destination, including the tag",
			},
		},
	}
}

func imagesyncCreate(d *schema.ResourceData, m interface{}) error {
	src := d.Get("source_digest").(string)
	srcImg, err := remote.GetImage(src)
	if err != nil {
		return err
	}

	dest := d.Get("destination_tag").(string)
	if err := remote.SyncImage(srcImg, dest); err != nil {
		return fmt.Errorf("sync: unable to write image '%s' to destination '%s'. err: %v", src, dest, err)
	}

	return imagesyncRead(d, m) // resync the state to ensure the digest and ID of state match remote img
}

func imagesyncRead(d *schema.ResourceData, meta interface{}) error {
	dest := d.Get("destination_tag").(string)
	destImg, err := remote.GetImage(dest)
	if err != nil {
		if errors.Is(err, remote.ErrImageNotFound) {
			d.SetId("")
			return nil
		}
		return err
	}

	imgID, err := ref.ImageID(dest, destImg)
	if err != nil {
		return err
	}
	d.SetId(imgID)

	return nil
}

func imagesyncDelete(d *schema.ResourceData, m interface{}) error {
	dest := d.Get("destination_tag").(string)

	// Delete the destination tag. Perform this first, regardless of if other tags exist
	if err := remote.DeleteImage(dest); err != nil {
		if errors.Is(err, remote.ErrImageNotFound) {
			return nil
		}

		return err
	}

	// If our destination tag was the last to reference this digest, a dangling image will be left behind.
	// We should try and delete this too; this matches the behavior of docker rmi.
	r, err := ref.StrictReference(dest)
	if err != nil {
		fmt.Printf("delete: error checking for dangling images, manual cleanup may be required. err: %v", err)
	}

	tags, err := remote.ListTags(r.Context().Name())
	if err != nil {
		fmt.Printf("delete: error checking for dangling images, manual cleanup may be required. err: %v", err)
		return nil
	}

	for _, t := range tags {
		digest, err := remote.GetDigest(r.Context().Name() + ":" + t)
		if errors.Is(err, remote.ErrImageNotFound) {
			continue // if this tag no longer exists, it can't be a dangling image
		}

		if digest == ref.DigestFromReference(d.Id()) {
			return nil // another image is using the same layers as we are, do not delete this manifest
		}
	}

	// No other tag in the remote shares this digest, we can delete this manifest
	if err := remote.DeleteImage(d.Id()); err != nil {
		if errors.Is(err, remote.ErrImageNotFound) {
			return nil
		}

		return err
	}

	return nil
}
