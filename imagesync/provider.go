package imagesync

import (
	"github.com/hashicorp/terraform/helper/schema"
	"github.com/hashicorp/terraform/terraform"
)

func Provider() terraform.ResourceProvider {
	return &schema.Provider{
		Schema:        map[string]*schema.Schema{},
		ConfigureFunc: nil,
		ResourcesMap: map[string]*schema.Resource{
			"imagesync": imagesync(),
		},
	}
}
