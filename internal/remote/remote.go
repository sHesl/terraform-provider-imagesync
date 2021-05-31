package remote

import (
	"errors"
	"fmt"

	"github.com/google/go-containerregistry/pkg/name"
	v1 "github.com/google/go-containerregistry/pkg/v1"
	"github.com/google/go-containerregistry/pkg/v1/empty"
	"github.com/google/go-containerregistry/pkg/v1/remote"
	"github.com/google/go-containerregistry/pkg/v1/remote/transport"
)

var (
	ErrImageNotFound = errors.New("image not found")
	uao              = remote.WithUserAgent("terraform-provider-imagesync")
)

// GetImage takes the ref (which may be either a tag or a digest), and returns an object representing
// an OCI image from the remote registry.
func GetImage(ref string) (v1.Image, error) {
	r, err := name.ParseReference(ref, name.StrictValidation)
	if err != nil {
		return empty.Image, err
	}

	ao, err := authOption(r)
	if err != nil {
		return empty.Image, err
	}

	i, err := remote.Image(r, ao, uao)
	if err != nil {
		if tErr, ok := (err).(*transport.Error); ok && tErr.StatusCode == 404 {
			return empty.Image, fmt.Errorf("'%s' not present in remote. %w", ref, ErrImageNotFound)
		}
		return empty.Image, err
	}

	return i, nil
}

// SyncImage copies the provided OCI image to the specified ref (which may be either a tag or a digest)
func SyncImage(img v1.Image, ref string) error {
	r, err := name.ParseReference(ref, name.StrictValidation)
	if err != nil {
		return err
	}

	ao, err := authOption(r)
	if err != nil {
		return err
	}

	return remote.Write(r, img, ao, uao)
}

// DeleteImage deletes the provided OCI image to the specified ref (which may be either a tag or a digest)
func DeleteImage(ref string) error {
	r, err := name.ParseReference(ref, name.StrictValidation)
	if err != nil {
		return err
	}

	ao, err := authOption(r)
	if err != nil {
		return err
	}

	if err := remote.Delete(r, ao, uao); err != nil {
		if tErr, ok := (err).(*transport.Error); ok && tErr.StatusCode == 404 {
			return fmt.Errorf("'%s' not present in remote. %w", ref, ErrImageNotFound)
		}
		return err
	}

	return nil
}

func ListTags(ref string) ([]string, error) {
	r, err := name.ParseReference(ref, name.WeakValidation)
	if err != nil {
		return nil, err
	}

	destAuthOpt, err := authOption(r)
	if err != nil {
		return nil, err
	}

	tags, err := remote.List(r.Context(), destAuthOpt, uao)
	if err != nil {
		return nil, err
	}

	return tags, nil
}

func GetDigest(ref string) (string, error) {
	i, err := GetImage(ref)
	if err != nil {
		return "", err
	}

	digest, err := i.Digest()
	if err != nil {
		return "", err
	}

	return digest.String(), nil
}
