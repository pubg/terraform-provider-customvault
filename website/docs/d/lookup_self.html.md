---
layout: "vault"
page_title: "CustomVault: customvault_lookup_self data source"
sidebar_current: "docs-vault-datasource-lookup-self"
description: |- Read Provider Uses Vault Token
---

# customvault\_lookup\_self

Reads Current Token Status.

## Example Usage

```hcl
# generally, these blocks would be in a different module
data "customvault_lookup_self" "self" {
}

output "temp_token" {
  value     = data.customvault_lookup_self.self.token
  sensitive = true
}

output "temp_token_data" {
  value     = jsondecode(data.customvault_lookup_self.self.data)
  sensitive = true
}
```

## Attributes Reference

In addition to the arguments above, the following attributes are exported:

* `token` - Current Vault Access Token, It might be temporary lifespan token.

* `data` - Current Token's Status. It would be raw json.
