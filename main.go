package main

import (
	"context"
	"flag"
	"log"

	"github.com/hashicorp/terraform-plugin-sdk/v2/plugin"
	"github.com/pubg/terraform-provider-customvault/v3/vault"
)

func main() {
	var debugMode bool
	flag.BoolVar(&debugMode, "debug", false, "set to true to run the provider with support for debuggers like delve")
	flag.Parse()

	opts := &plugin.ServeOpts{ProviderFunc: vault.Provider}

	if debugMode {
		err := plugin.Debug(context.Background(), "pubg/customvault", opts)
		if err != nil {
			log.Fatal(err.Error())
		}
		return
	}

	plugin.Serve(opts)
}
