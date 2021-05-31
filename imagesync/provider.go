package imagesync

import (
	"github.com/hashicorp/terraform-plugin-sdk/helper/schema"
	"github.com/hashicorp/terraform-plugin-sdk/terraform"
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
