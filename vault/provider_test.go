package vault

import (
	"fmt"
	"io/ioutil"
	"os"
	"path"
	"testing"

	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
	"github.com/hashicorp/vault/command/config"
)

// How to run the acceptance tests for this provider:
//
// - Obtain an official Vault release from the Vault website at
//   https://vaultproject.io/ and extract the "vault" binary
//   somewhere.
//
// - Run the following to start the Vault server in development mode:
//       vault server -dev
//
// - Take the "Root Token" value printed by Vault as the server started
//   up and set it as the value of the VAULT_TOKEN environment variable
//   in a new shell whose current working directory is the root of the
//   Terraform repository.
//
// - As directed by the Vault server output, set the VAULT_ADDR environment
//   variable. e.g.:
//       export VAULT_ADDR='http://127.0.0.1:8200'
//
// - Run the Terraform acceptance tests as usual:
//       make testacc TEST=./builtin/providers/vault
//
// The tests expect to be run in a fresh, empty Vault and thus do not attempt
// to randomize or otherwise make the generated resource paths unique on
// each run. In case of weird behavior, restart the Vault dev server to
// start over with a fresh Vault. (Remember to reset VAULT_TOKEN.)

func TestProvider(t *testing.T) {
	if err := Provider().InternalValidate(); err != nil {
		t.Fatalf("err: %s", err)
	}
}

var testProvider *schema.Provider
var testProviders map[string]*schema.Provider

func init() {
	testProvider = Provider()
	testProviders = map[string]*schema.Provider{
		"customvault": testProvider,
	}
}

func testAccPreCheck(t *testing.T) {
	if v := os.Getenv("VAULT_ADDR"); v == "" {
		t.Fatal("VAULT_ADDR must be set for acceptance tests")
	}
	if v := os.Getenv("VAULT_TOKEN"); v == "" {
		t.Fatal("VAULT_TOKEN must be set for acceptance tests")
	}
}

// A basic token helper script.
const tokenHelperScript = `#!/usr/bin/env bash
echo "helper-token"
`

// A token helper script that echos back the VAULT_ADDR value
const echoBackTokenHelperScript = `#!/usr/bin/env bash
printenv VAULT_ADDR
`

func failIfErr(t *testing.T, f func() error) {
	if err := f(); err != nil {
		t.Fatal(err)
	}
}

// tempUnsetenv is the equivalent of calling `os.Unsetenv` but returns
// a function that be called to restore the modified environment variable
// to its state prior to this function being called.
// The reset function will never be nil.
func tempUnsetenv(key string) (reset func() error, err error) {
	reset = resetEnvFunc(key)
	err = os.Unsetenv(key)
	return reset, err
}

// tempSetenv is the equivalent of calling `os.Setenv` but returns
// a function that be called to restore the modified environment variable
// to its state prior to this function being called.
// The reset function will never be nil.
func tempSetenv(key string, value string) (reset func() error, err error) {
	reset = resetEnvFunc(key)
	err = os.Setenv(key, value)
	return reset, err
}

// resetEnvFunc returns a func that will reset the state of
// the environment variable named `key` when it is called to the
// state captured at the time the function was created
func resetEnvFunc(key string) (reset func() error) {
	if current, exists := os.LookupEnv(key); exists {
		return func() error {
			return os.Setenv(key, current)
		}
	} else {
		return func() error {
			return os.Unsetenv(key)
		}
	}
}

// setupTestTokenHelper creates a temporary vault config that uses the provided
// script as a token helper and returns a cleanup function that should be deferred and
// called to set back the environment to how it was were pre test.
func setupTestTokenHelper(t *testing.T, script string) (cleanup func()) {
	// Use a temp dir for test files.
	dir, err := ioutil.TempDir("", "terraform-provider-vault")
	if err != nil {
		t.Fatal(err)
	}

	// Write out the config file and helper script file.
	configPath := path.Join(dir, "vault-config")
	helperPath := path.Join(dir, "helper-script")
	configStr := fmt.Sprintf(`token_helper = "%s"`, helperPath)
	err = ioutil.WriteFile(configPath, []byte(configStr), 0666)
	if err != nil {
		t.Fatal(err)
	}
	err = ioutil.WriteFile(helperPath, []byte(script), 0777)
	if err != nil {
		t.Fatal(err)
	}
	// Point Vault at the config file.
	os.Setenv(config.ConfigPathEnv, configPath)
	if err != nil {
		t.Fatal(err)
	}

	return func() {
		if err := os.Unsetenv(config.ConfigPathEnv); err != nil {
			t.Fatal(err)
		}

		if err := os.RemoveAll(dir); err != nil {
			t.Fatal(err)
		}
	}
}
