---
layout: "vault"
page_title: "CustomVault: customvault_tencent_access_credentials data source"
sidebar_current: "docs-vault-datasource-tencent-access-credentials"
description: |- Reads Tencent credentials from an Tencent secret backend in Vault
---

# customvault\_tencent\_access\_credentials

Reads Tencent credentials from an Tencent secret backend in Vault.

## Example Usage

```terraform
# generally, these blocks would be in a different module
data "customvault_tencent_access_credentials" "creds" {
  backend    = "tencent"
  role       = "my-terraform-provisioner"
  sts_region = "ap-seoul"
}

provider "tencentcloud" {
  access_key     = data.customvault_tencent_access_credentials.creds.access_key
  secret_key     = data.customvault_tencent_access_credentials.creds.secret_key
  security_token = data.customvault_tencent_access_credentials.creds.security_token
}

output "arn" {
  value = data.customvault_tencent_access_credentials.creds.arn
}
```

## Argument Reference

The following arguments are supported:

* `backend` - (Required) The path to the Tencent secret backend to read credentials from, with no leading or
  trailing `/`s.

* `role` - (Required) The name of the Tencent secret backend role to read credentials from, with no leading or
  trailing `/`s.

* `sts_region` - (Optional) TencentCloud Region for use Credential Validation. Default is `ap-guangzhou`

## Attributes Reference

In addition to the arguments above, the following attributes are exported:

* `access_key` - The Tencent Access Key ID returned by Vault.

* `secret_key` - The Tencent Secret Key returned by Vault.

* `security_token` - The STS token returned by Vault.

* `lease_id` - The lease identifier assigned by Vault.

* `lease_duration` - The duration of the secret lease, in seconds relative to the time the data was requested. Once this
  time has passed any plan generated with this data may fail to apply.

* `lease_start_time` - As a convenience, this records the current time on the computer where Terraform is running when
  the data is requested. This can be used to approximate the absolute time represented by
  `lease_duration`, though users must allow for any clock drift and response latency relative to the Vault server.

* `lease_renewable` - `true` if the lease can be renewed using Vault's
  `sys/renew/{lease-id}` endpoint. Terraform does not currently support lease renewal, and so it will request a new
  lease each time this data source is refreshed.

* `arn` - TencentCloud CAM Identity ARN.
