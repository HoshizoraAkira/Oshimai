// Command terraform-provider-oshimai is a minimal Terraform provider for managing recurring
// GameDay chaos/load schedules on an Oshimai server declaratively, alongside the rest of your
// infrastructure — e.g. spin up a "quarterly checkout GameDay" resource next to the checkout
// service's own Terraform-managed infra.
package main

import (
	"context"
	"log"

	"github.com/hashicorp/terraform-plugin-framework/providerserver"

	"github.com/oshimai/terraform-provider-oshimai/internal/provider"
)

func main() {
	err := providerserver.Serve(context.Background(), provider.New, providerserver.ServeOpts{
		Address: "registry.terraform.io/oshimai/oshimai",
	})
	if err != nil {
		log.Fatal(err)
	}
}
