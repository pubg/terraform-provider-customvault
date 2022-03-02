package vault

import (
	"fmt"
	"os"
	"testing"

	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/resource"
)

func TestTencentAccessCredentialsDataSource(t *testing.T) {
	tcAccountId := os.Getenv("TC_ACCOUNTID")
	if tcAccountId == "" {
		t.Fatal("TC_ACCOUNTID must be set for acceptance tests")
	}

	resource.Test(t, resource.TestCase{
		PreCheck: func() {
			testAccPreCheck(t)
		},
		Providers: testProviders,
		Steps: []resource.TestStep{
			{
				Config: testTencentAccessCredentialsDataSourceConfig(tcAccountId),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttrSet("data.customvault_tencent_access_credentials.tc", "access_key"),
					resource.TestCheckResourceAttrSet("data.customvault_tencent_access_credentials.tc", "secret_key"),
					resource.TestCheckResourceAttrSet("data.customvault_tencent_access_credentials.tc", "security_token"),
					resource.TestCheckResourceAttrSet("data.customvault_tencent_access_credentials.tc", "arn"),
				),
			},
		},
	})
}

func testTencentAccessCredentialsDataSourceConfig(accountId string) string {
	return fmt.Sprintf(`
data "customvault_tencent_access_credentials" "tc" {
  backend = "tencentcloud/%s"
  role    = "xtrm-terraform-provisioner"
  sts_region = "ap-seoul"
}
`, accountId)
}
