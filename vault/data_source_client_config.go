package vault

import (
	"github.com/hashicorp/terraform-plugin-sdk/helper/schema"
	"github.com/hashicorp/vault/api"
)

func clientConfigDataSource() *schema.Resource {
	return &schema.Resource{
		Read: clientConfigDataSourceRead,
		Schema: map[string]*schema.Schema{
			"token": {
				Type:        schema.TypeString,
				Computed:    true,
				Sensitive:   true,
				Description: "Vault Terraform Client Token",
			},
		},
	}
}

func clientConfigDataSourceRead(d *schema.ResourceData, meta interface{}) error {
	client := meta.(*api.Client)

	d.SetId("static-id")
	d.Set("token", client.Token())
	return nil
}
