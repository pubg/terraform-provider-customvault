package vault

import (
	"fmt"
	"testing"

	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/resource"
)

func TestLookupSelfDataSource(t *testing.T) {
	resource.Test(t, resource.TestCase{
		PreCheck: func() {
			testAccPreCheck(t)
		},
		Providers: testProviders,
		Steps: []resource.TestStep{
			{
				Config: testLookupSelfDataSourceConfig(),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttrSet("data.customvault_lookup_self.self", "token"),
					resource.TestCheckResourceAttrSet("data.customvault_lookup_self.self", "data"),
				),
			},
		},
	})
}

func testLookupSelfDataSourceConfig() string {
	return fmt.Sprintf(`
data "customvault_lookup_self" "self" {
}
`)
}
