package vault

import (
	"encoding/json"

	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
	"github.com/hashicorp/vault/api"
)

func lookupSelfDataSource() *schema.Resource {
	return &schema.Resource{
		Read: lookupSelfDataSourceRead,
		Schema: map[string]*schema.Schema{
			"token": {
				Type:        schema.TypeString,
				Computed:    true,
				Sensitive:   true,
				Description: "Vault Terraform Client Token",
			},
			"data": {
				Type:        schema.TypeString,
				Computed:    true,
				Sensitive:   true,
				Description: "Vault Terraform Client Token",
			},
		},
	}
}

func lookupSelfDataSourceRead(d *schema.ResourceData, meta interface{}) error {
	client := meta.(*api.Client)

	d.SetId("static-id")
	d.Set("token", client.Token())

	tokenInfo, err := client.Auth().Token().LookupSelf()
	if err != nil {
		return err
	}

	buf, err := json.Marshal(tokenInfo.Data)
	if err != nil {
		return err
	}
	d.Set("data", string(buf))
	return nil
}
