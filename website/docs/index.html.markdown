---
layout: "vault"
page_title: "Provider: CustomVault"
sidebar_current: "docs-vault-index"
description: |- The Vault provider allows Terraform to read from, write to, and configure HashiCorp Vault
---

# Vault Provider

The Vault provider allows Terraform to read from, write to, and configure
[HashiCorp Vault](https://vaultproject.io/).

# Configure Provider

Same as [Hashicorp Vault Provider v3.3.1](https://registry.terraform.io/providers/hashicorp/vault/latest/docs)

```terraform
provider "customvault" {
  # It is strongly recommended to configure this provider through the
  # environment variables described above, so that each user can have
  # separate credentials set in the environment.
  #
  # This will default to using $VAULT_ADDR
  # But can be set explicitly
  # address = "https://vault.example.net:8200"
}

data "customvault_lookup_self" "self" {

}

output "temp_token" {
  value     = data.customvault_lookup_self.self.token
  sensitive = true
}

```
