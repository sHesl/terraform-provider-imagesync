package ref

import (
	"errors"
	"fmt"
	"strings"

	"github.com/google/go-containerregistry/pkg/name"
	v1 "github.com/google/go-containerregistry/pkg/v1"
)

var ErrInvalidReference = errors.New("references must specify a digest")

func StrictReference(ref string) (name.Reference, error) {
	r, err := name.ParseReference(ref, name.StrictValidation)
	if err != nil {
		return nil, fmt.Errorf("invalid reference '%s'. %w", ref, ErrInvalidReference)
	}

	return r, nil
}

// ImageID returns the fully-qualified reference to the image, with any tags replaced with the sha256 digest.
// Here, we mostly rely on validation done in ParseReference (which is also performed on remote read/writes)
// to ensure the reference is valid.
func ImageID(ref string, img v1.Image) (string, error) {
	r, err := StrictReference(ref)
	if err != nil {
		return "", err
	}

	digest, err := img.Digest()
	if err != nil {
		return "", err
	}

	return r.Context().Name() + "@" + digest.String(), nil
}

// DigestFromReference strips all content preceding the digest for the given, fully-qualified, reference.
func DigestFromReference(ref string) string {
	if _, err := StrictReference(ref); err != nil {
		return ""
	}

	at := strings.LastIndex(ref, "@")
	if at == -1 {
		return ""
	}

	if at+1 >= len(ref) {
		return ""
	}

	return ref[at+1:]
}
