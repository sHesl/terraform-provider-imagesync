package remote

import (
	"github.com/google/go-containerregistry/pkg/authn"
	"github.com/google/go-containerregistry/pkg/name"
	"github.com/google/go-containerregistry/pkg/v1/google"
	"github.com/google/go-containerregistry/pkg/v1/remote"
)

func authOption(ref name.Reference) (remote.Option, error) {
	switch {
	case registryIn(ref.Context().Registry, "gcr.io", "eu.gcr.io", "us.gcr.io", "asia.gcr.io"):
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
