package ref_test

import (
	"errors"
	"testing"

	"github.com/google/go-containerregistry/pkg/v1/empty"
	"github.com/sHesl/terraform-provider-imagesync/internal/ref"
)

func TestImageID(t *testing.T) {
	emptyDigest, _ := empty.Image.Digest()

	type testCase struct {
		input  string
		exp    string
		expErr error
	}

	testCases := []testCase{
		{
			input: "127.0.0.1:57691/blah:1.0",
			exp:   "127.0.0.1:57691/blah@" + emptyDigest.String(),
		},
		{
			input: "remote.com/v2/blah:latest",
			exp:   "remote.com/v2/blah@" + emptyDigest.String(),
		},
		{
			input: "docker-daemon://remote.com/v2/blah:latest",
			exp:   "docker-daemon://remote.com/v2/blah@" + emptyDigest.String(),
		},
		{
			// We don't support references that specify protocols in the main provider, but here we are testing the
			// we are parsing urls correctly, yet only acting on the path
			input: "https://remote/v2/blah:1.1.1",
			exp:   "https://remote/v2/blah@" + emptyDigest.String(),
		},
		{
			// Prefer image digest to reference digest
			input: "remote.com/blah@sha256:aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa",
			exp:   "remote.com/blah@" + emptyDigest.String(),
		},
		{
			input:  "remote.com/v2/blah", // no tag or digest, should fail strict validation during reference parse
			exp:    "",
			expErr: ref.ErrInvalidReference,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.input, func(t *testing.T) {
			result, err := ref.ImageID(tc.input, empty.Image)
			if err != nil && tc.expErr == nil {
				t.Fatalf("unexpected error from input '%s'. err: %v", tc.input, err)
			}

			if tc.exp != result {
				t.Fatalf("expected: '%s'. got '%s'", tc.exp, result)
			}

			if tc.expErr != nil && err != nil && (tc.expErr != err && !errors.Is(err, tc.expErr)) {
				t.Fatalf("expected error '%v'. got: %v", tc.expErr, err)
			}

			if tc.expErr != nil && err == nil {
				t.Fatalf("expected err '%v'. got no error", tc.expErr)
			}
		})
	}
}

func TestDigestFromReference(t *testing.T) {
	type testCase struct {
		input string
		exp   string
	}

	testCases := []testCase{
		{
			input: "remote/v2/blah:not_a_digest",
			exp:   "",
		},
		{
			input: "remote/v2/blah@",
			exp:   "",
		},
		{
			input: "remote/v2/blah@sha256:123",
			exp:   "sha256:123",
		},

		// We don't support references that specify protocols in the main provider, but this next test proves we
		// are considering only the final : in the input.
		{
			input: "https://remote/v2/blah@sha256:123",
			exp:   "sha256:123",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.input, func(t *testing.T) {
			result := ref.DigestFromReference(tc.input)
			if tc.exp != result {
				t.Fatalf("expected: '%s'. got '%s'", tc.exp, result)
			}
		})
	}
}
