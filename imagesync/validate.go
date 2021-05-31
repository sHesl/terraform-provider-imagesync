package imagesync

import (
	"fmt"
	"regexp"
	"strings"

	"github.com/google/go-containerregistry/pkg/name"
)

// Regexp taken from https://github.com/opencontainers/go-digest/blob/master/digest.go#L63
var digestRegexp = regexp.MustCompile(`[a-z0-9]+(?:[.+_-][a-z0-9]+)*:[a-zA-Z0-9=_-]+`)

func validateReference(refI interface{}, fieldName string) ([]string, []error) {
	ref, ok := refI.(string)
	if !ok {
		return nil, []error{fmt.Errorf("'%s' must be a string. got %#v", fieldName, refI)}
	}

	if ref == "" {
		return nil, []error{fmt.Errorf("'%s' field cannot be empty", fieldName)}
	}

	if _, err := name.ParseReference(ref, name.StrictValidation); err != nil {
		return nil, []error{fmt.Errorf("'%s' is not a valid reference. err: %v", ref, err)}
	}

	return nil, nil
}

func validateTag(refI interface{}, fieldName string) ([]string, []error) {
	if msgs, errs := validateReference(refI, fieldName); errs != nil {
		return msgs, errs
	}

	r, _ := name.ParseReference(refI.(string), name.StrictValidation) // prior validation makes this safe

	if strings.HasPrefix(r.Identifier(), "sha") {
		return nil, []error{fmt.Errorf("invalid tag '%s'. must not reference a digest algorithm", r.String())}
	}

	return nil, nil
}

func validateDigest(refI interface{}, fieldName string) ([]string, []error) {
	if msgs, errs := validateReference(refI, fieldName); errs != nil {
		return msgs, errs
	}

	r, _ := name.ParseReference(refI.(string), name.StrictValidation) // prior validation makes this safe

	if matched := digestRegexp.MatchString(r.Identifier()); !matched {
		return nil, []error{fmt.Errorf("invalid digest '%s'. must match '%s'", r.String(), digestRegexp.String())}
	}

	return nil, nil
}
