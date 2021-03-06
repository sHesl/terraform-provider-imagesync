package main

import (
	"github.com/hashicorp/terraform/plugin"
	"github.com/sHesl/terraform-provider-imagesync/imagesync"
)

func main() {
	plugin.Serve(&plugin.ServeOpts{ProviderFunc: imagesync.Provider})
}
