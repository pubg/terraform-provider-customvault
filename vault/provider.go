package vault

import (
	"errors"
	"fmt"
	"log"
	"net/http"
	"os"
	"strings"

	"github.com/hashicorp/go-multierror"
	"github.com/hashicorp/terraform-plugin-sdk/helper/logging"
	"github.com/hashicorp/terraform-plugin-sdk/helper/mutexkv"
	"github.com/hashicorp/terraform-plugin-sdk/helper/schema"
	"github.com/hashicorp/vault/api"
	awsauth "github.com/hashicorp/vault/builtin/credential/aws"
	"github.com/hashicorp/vault/command/config"
)

const (
	// GenericPath is used for inventorying paths that can be used for
	// multiple endpoints in Vault.
	GenericPath = "generic"

	// UnknownPath is used for inventorying paths that have no obvious
	// current endpoint they serve in Vault, and may relate to previous
	// versions of Vault.
	// We aim to deprecate items in this category.
	UnknownPath = "unknown"
)

// This is a global MutexKV for use within this provider.
// Use this when you need to have multiple resources or even multiple instances
// of the same resource write to the same path in Vault.
// The key of the mutex should be the path in Vault.
var vaultMutexKV = mutexkv.NewMutexKV()

func Provider() *schema.Provider {
	dataSourcesMap, err := parse(DataSourceRegistry)
	if err != nil {
		panic(err)
	}
	resourcesMap, err := parse(ResourceRegistry)
	if err != nil {
		panic(err)
	}
	return &schema.Provider{
		Schema: map[string]*schema.Schema{
			"address": {
				Type:        schema.TypeString,
				Required:    true,
				DefaultFunc: schema.EnvDefaultFunc("VAULT_ADDR", nil),
				Description: "URL of the root of the target Vault server.",
			},
			"add_address_to_env": {
				Type:        schema.TypeString,
				Optional:    true,
				Default:     false,
				Description: "If true, adds the value of the `address` argument to the Terraform process environment.",
			},
			"token": {
				Type:        schema.TypeString,
				Required:    true,
				DefaultFunc: schema.EnvDefaultFunc("VAULT_TOKEN", ""),
				Description: "Token to use to authenticate to Vault.",
			},
			"token_name": {
				Type:        schema.TypeString,
				Optional:    true,
				DefaultFunc: schema.EnvDefaultFunc("VAULT_TOKEN_NAME", ""),
				Description: "Token name to use for creating the Vault child token.",
			},
			"ca_cert_file": {
				Type:        schema.TypeString,
				Optional:    true,
				DefaultFunc: schema.EnvDefaultFunc("VAULT_CACERT", ""),
				Description: "Path to a CA certificate file to validate the server's certificate.",
			},
			"ca_cert_dir": {
				Type:        schema.TypeString,
				Optional:    true,
				DefaultFunc: schema.EnvDefaultFunc("VAULT_CAPATH", ""),
				Description: "Path to directory containing CA certificate files to validate the server's certificate.",
			},
			"auth_login": {
				Type:        schema.TypeList,
				Optional:    true,
				Description: "Login to vault with an existing auth method using auth/<mount>/login",
				Elem: &schema.Resource{
					Schema: map[string]*schema.Schema{
						"path": {
							Type:     schema.TypeString,
							Required: true,
						},
						"namespace": {
							Type:     schema.TypeString,
							Optional: true,
						},
						"parameters": {
							Type:     schema.TypeMap,
							Optional: true,
							Elem: &schema.Schema{
								Type: schema.TypeString,
							},
						},
						"method": {
							Type:     schema.TypeString,
							Optional: true,
						},
					},
				},
			},
			"client_auth": {
				Type:        schema.TypeList,
				Optional:    true,
				Description: "Client authentication credentials.",
				Elem: &schema.Resource{
					Schema: map[string]*schema.Schema{
						"cert_file": {
							Type:        schema.TypeString,
							Required:    true,
							DefaultFunc: schema.EnvDefaultFunc("VAULT_CLIENT_CERT", ""),
							Description: "Path to a file containing the client certificate.",
						},
						"key_file": {
							Type:        schema.TypeString,
							Required:    true,
							DefaultFunc: schema.EnvDefaultFunc("VAULT_CLIENT_KEY", ""),
							Description: "Path to a file containing the private key that the certificate was issued for.",
						},
					},
				},
			},
			"skip_tls_verify": {
				Type:        schema.TypeBool,
				Optional:    true,
				DefaultFunc: schema.EnvDefaultFunc("VAULT_SKIP_VERIFY", false),
				Description: "Set this to true only if the target Vault server is an insecure development instance.",
			},
			"max_lease_ttl_seconds": {
				Type:     schema.TypeInt,
				Optional: true,

				// Default is 20min, which is intended to be enough time for
				// a reasonable Terraform run can complete but not
				// significantly longer, so that any leases are revoked shortly
				// after Terraform has finished running.
				DefaultFunc: schema.EnvDefaultFunc("TERRAFORM_VAULT_MAX_TTL", 1200),

				Description: "Maximum TTL for secret leases requested by this provider",
			},
			"max_retries": {
				Type:     schema.TypeInt,
				Optional: true,

				DefaultFunc: schema.EnvDefaultFunc("VAULT_MAX_RETRIES", 2),
				Description: "Maximum number of retries when a 5xx error code is encountered.",
			},
			"namespace": {
				Type:        schema.TypeString,
				Optional:    true,
				DefaultFunc: schema.EnvDefaultFunc("VAULT_NAMESPACE", ""),
				Description: "The namespace to use. Available only for Vault Enterprise",
			},
			"headers": {
				Type:        schema.TypeList,
				Optional:    true,
				Sensitive:   true,
				Description: "The headers to send with each Vault request.",
				Elem: &schema.Resource{
					Schema: map[string]*schema.Schema{
						"name": {
							Type:        schema.TypeString,
							Required:    true,
							Description: "The header name",
						},
						"value": {
							Type:        schema.TypeString,
							Required:    true,
							Description: "The header value",
						},
					},
				},
			},
		},
		ConfigureFunc:  providerConfigure,
		DataSourcesMap: dataSourcesMap,
		ResourcesMap:   resourcesMap,
	}
}

// Description is essentially a DataSource or Resource with some additional metadata
// that helps with maintaining the Terraform Vault Provider.
type Description struct {
	// PathInventory is used for taking an inventory of the supported endpoints in the
	// Terraform Vault Provider and comparing them to the endpoints noted as available in
	// Vault's OpenAPI description. A list of Vault's endpoints can be obtained by,
	// from Vault's home directory, running "$ ./scripts/gen_openapi.sh", and then by
	// drilling into the paths with "$ cat openapi.json | jq ".paths" | jq 'keys[]'".
	// Here's a short example of how paths and their path variables should be represented:
	//		"/transit/keys/{name}/config"
	//		"/transit/random"
	//		"/transit/random/{urlbytes}"
	//		"/transit/sign/{name}/{urlalgorithm}"
	PathInventory []string

	// EnterpriseOnly defaults to false, but should be marked true if a resource is enterprise only.
	EnterpriseOnly bool

	Resource *schema.Resource
}

var (
	DataSourceRegistry = map[string]*Description{
		"customvault_tencent_access_credentials": {
			Resource:      tencentAccessCredentialsDataSource(),
			PathInventory: []string{"/tencentsecrets/creds"},
		},
	}

	ResourceRegistry = map[string]*Description{
	}
)

func providerToken(d *schema.ResourceData) (string, error) {
	if token := d.Get("token").(string); token != "" {
		return token, nil
	}

	if addAddr := d.Get("add_address_to_env").(string); addAddr == "true" {
		if addr := d.Get("address").(string); addr != "" {
			if current, exists := os.LookupEnv("VAULT_ADDR"); exists {
				defer func() {
					os.Setenv("VAULT_ADDR", current)
				}()
			} else {
				defer func() {
					os.Unsetenv("VAULT_ADDR")
				}()
			}
			os.Setenv("VAULT_ADDR", addr)
		}
	}

	// Use ~/.vault-token, or the configured token helper.
	tokenHelper, err := config.DefaultTokenHelper()
	if err != nil {
		return "", fmt.Errorf("error getting token helper: %s", err)
	}
	token, err := tokenHelper.Get()
	if err != nil {
		return "", fmt.Errorf("error getting token: %s", err)
	}
	return strings.TrimSpace(token), nil
}

func providerConfigure(d *schema.ResourceData) (interface{}, error) {
	clientConfig := api.DefaultConfig()
	addr := d.Get("address").(string)
	if addr != "" {
		clientConfig.Address = addr
	}

	clientAuthI := d.Get("client_auth").([]interface{})
	if len(clientAuthI) > 1 {
		return nil, fmt.Errorf("client_auth block may appear only once")
	}

	clientAuthCert := ""
	clientAuthKey := ""
	if len(clientAuthI) == 1 {
		clientAuth := clientAuthI[0].(map[string]interface{})
		clientAuthCert = clientAuth["cert_file"].(string)
		clientAuthKey = clientAuth["key_file"].(string)
	}

	err := clientConfig.ConfigureTLS(&api.TLSConfig{
		CACert:   d.Get("ca_cert_file").(string),
		CAPath:   d.Get("ca_cert_dir").(string),
		Insecure: d.Get("skip_tls_verify").(bool),

		ClientCert: clientAuthCert,
		ClientKey:  clientAuthKey,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to configure TLS for Vault API: %s", err)
	}

	clientConfig.HttpClient.Transport = logging.NewTransport("Vault", clientConfig.HttpClient.Transport)

	client, err := api.NewClient(clientConfig)
	if err != nil {
		return nil, fmt.Errorf("failed to configure Vault API: %s", err)
	}

	client.SetCloneHeaders(true)

	// Set headers if provided
	headers := d.Get("headers").([]interface{})
	parsedHeaders := client.Headers().Clone()

	if parsedHeaders == nil {
		parsedHeaders = make(http.Header)
	}

	for _, h := range headers {
		header := h.(map[string]interface{})
		if name, ok := header["name"]; ok {
			parsedHeaders.Add(name.(string), header["value"].(string))
		}
	}
	client.SetHeaders(parsedHeaders)

	client.SetMaxRetries(d.Get("max_retries").(int))

	// Try an get the token from the config or token helper
	token, err := providerToken(d)
	if err != nil {
		return nil, err
	}

	// Attempt to use auth/<mount>login if 'auth_login' is provided in provider config
	authLoginI := d.Get("auth_login").([]interface{})
	if len(authLoginI) > 1 {
		return "", fmt.Errorf("auth_login block may appear only once")
	}

	if len(authLoginI) == 1 {
		authLogin := authLoginI[0].(map[string]interface{})
		authLoginPath := authLogin["path"].(string)
		authLoginNamespace := ""
		if authLoginNamespaceI, ok := authLogin["namespace"]; ok {
			authLoginNamespace = authLoginNamespaceI.(string)
			client.SetNamespace(authLoginNamespace)
		}
		authLoginParameters := authLogin["parameters"].(map[string]interface{})

		method := authLogin["method"].(string)
		if method == "aws" {
			if err := signAWSLogin(authLoginParameters); err != nil {
				return nil, fmt.Errorf("error signing AWS login request: %s", err)
			}
		}

		secret, err := client.Logical().Write(authLoginPath, authLoginParameters)
		if err != nil {
			return nil, err
		}
		token = secret.Auth.ClientToken
	}
	if token != "" {
		client.SetToken(token)
	}
	if client.Token() == "" {
		return nil, errors.New("no vault token found")
	}

	tokenName := d.Get("token_name").(string)
	if tokenName == "" {
		tokenName = "terraform"
	}

	// In order to enforce our relatively-short lease TTL, we derive a
	// temporary child token that inherits all of the policies of the
	// token we were given but expires after max_lease_ttl_seconds.
	//
	// The intent here is that Terraform will need to re-fetch any
	// secrets on each run and so we limit the exposure risk of secrets
	// that end up stored in the Terraform state, assuming that they are
	// credentials that Vault is able to revoke.
	//
	// Caution is still required with state files since not all secrets
	// can explicitly be revoked, and this limited scope won't apply to
	// any secrets that are *written* by Terraform to Vault.

	// Set the namespace to the token's namespace only for the
	// child token creation
	tokenInfo, err := client.Auth().Token().LookupSelf()
	if err != nil {
		return nil, err
	}
	if tokenNamespaceRaw, ok := tokenInfo.Data["namespace_path"]; ok {
		tokenNamespace := tokenNamespaceRaw.(string)
		if tokenNamespace != "" {
			client.SetNamespace(tokenNamespace)
		}
	}

	renewable := false
	childTokenLease, err := client.Auth().Token().Create(&api.TokenCreateRequest{
		DisplayName:    tokenName,
		TTL:            fmt.Sprintf("%ds", d.Get("max_lease_ttl_seconds").(int)),
		ExplicitMaxTTL: fmt.Sprintf("%ds", d.Get("max_lease_ttl_seconds").(int)),
		Renewable:      &renewable,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to create limited child token: %s", err)
	}

	childToken := childTokenLease.Auth.ClientToken
	policies := childTokenLease.Auth.Policies

	log.Printf("[INFO] Using Vault token with the following policies: %s", strings.Join(policies, ", "))

	// Set the token to the generated child token
	client.SetToken(childToken)

	// Set the namespace to the requested namespace, if provided
	namespace := d.Get("namespace").(string)
	if namespace != "" {
		client.SetNamespace(namespace)
	}
	return client, nil
}

func parse(descs map[string]*Description) (map[string]*schema.Resource, error) {
	var errs error
	resourceMap := make(map[string]*schema.Resource)
	for k, desc := range descs {
		resourceMap[k] = desc.Resource
		if len(desc.PathInventory) == 0 {
			errs = multierror.Append(errs, fmt.Errorf("%q needs its paths inventoried", k))
		}
	}
	return resourceMap, errs
}

func signAWSLogin(parameters map[string]interface{}) error {
	var accessKey, secretKey, securityToken string
	if val, ok := parameters["aws_access_key_id"].(string); ok {
		accessKey = val
	}

	if val, ok := parameters["aws_secret_access_key"].(string); ok {
		secretKey = val
	}

	if val, ok := parameters["aws_security_token"].(string); ok {
		securityToken = val
	}

	creds, err := awsauth.RetrieveCreds(accessKey, secretKey, securityToken)
	if err != nil {
		return fmt.Errorf("failed to retrieve AWS credentials: %s", err)
	}

	var headerValue, stsRegion string
	if val, ok := parameters["header_value"].(string); ok {
		headerValue = val
	}

	if val, ok := parameters["sts_region"].(string); ok {
		stsRegion = val
	}

	loginData, err := awsauth.GenerateLoginData(creds, headerValue, stsRegion)
	if err != nil {
		return fmt.Errorf("failed to generate AWS login data: %s", err)
	}

	parameters["iam_http_request_method"] = loginData["iam_http_request_method"]
	parameters["iam_request_url"] = loginData["iam_request_url"]
	parameters["iam_request_headers"] = loginData["iam_request_headers"]
	parameters["iam_request_body"] = loginData["iam_request_body"]

	return nil
}
