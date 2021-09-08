package vault

import (
	"fmt"
	"log"
	"time"

	"github.com/hashicorp/terraform-plugin-sdk/helper/schema"
	"github.com/hashicorp/vault/api"
)

const (
	// sequentialSuccessesRequired is the number of times the test of an eventually consistent
	// credential must succeed before we return it for use.
	sequentialSuccessesRequired = 5

	// sequentialSuccessTimeLimit is how long we'll wait for eventually consistent AWS creds
	// to propagate before giving up. In real life, we've seen it take up to 15 seconds, so
	// this is ample and if it's unsuccessful there's something else wrong.
	sequentialSuccessTimeLimit = time.Minute

	// retryTimeOut is how long we'll wait before timing out when we're retrying credentials.
	// This corresponds to Vault's default 30-second request timeout.
	retryTimeOut = 30 * time.Second

	// propagationBuffer is the added buffer of time we'll wait after N sequential successes
	// before returning credentials for use.
	propagationBuffer = 5 * time.Second
)

func awsAccessCredentialsDataSource() *schema.Resource {
	return &schema.Resource{
		Read: awsAccessCredentialsDataSourceRead,

		Schema: map[string]*schema.Schema{
			"backend": {
				Type:        schema.TypeString,
				Required:    true,
				Description: "AWS Secret Backend to read credentials from.",
			},
			"role": {
				Type:        schema.TypeString,
				Required:    true,
				Description: "AWS Secret Role to read credentials from.",
			},
			"type": {
				Type:        schema.TypeString,
				Optional:    true,
				Default:     "creds",
				Description: "Type of credentials to read. Must be either 'creds' for Access Key and Secret Key, or 'sts' for STS.",
				ValidateFunc: func(v interface{}, k string) (ws []string, errs []error) {
					value := v.(string)
					if value != "sts" && value != "creds" {
						errs = append(errs, fmt.Errorf("type must be creds or sts"))
					}
					return nil, errs
				},
			},
			"role_arn": {
				Type:        schema.TypeString,
				Optional:    true,
				Description: "ARN to use if multiple are available in the role. Required if the role has multiple ARNs.",
			},
			"region": {
				Type:        schema.TypeString,
				Optional:    true,
				Description: "Region the read credentials belong to.",
			},
			"access_key": {
				Type:        schema.TypeString,
				Computed:    true,
				Description: "AWS access key ID read from Vault.",
			},

			"secret_key": {
				Type:        schema.TypeString,
				Computed:    true,
				Description: "AWS secret key read from Vault.",
			},

			"security_token": {
				Type:        schema.TypeString,
				Computed:    true,
				Description: "AWS security token read from Vault. (Only returned if type is 'sts').",
			},

			"lease_id": {
				Type:        schema.TypeString,
				Computed:    true,
				Description: "Lease identifier assigned by vault.",
			},

			"lease_duration": {
				Type:        schema.TypeInt,
				Computed:    true,
				Description: "Lease duration in seconds relative to the time in lease_start_time.",
			},

			"lease_start_time": {
				Type:        schema.TypeString,
				Computed:    true,
				Description: "Time at which the lease was read, using the clock of the system where Terraform was running",
			},

			"lease_renewable": {
				Type:        schema.TypeBool,
				Computed:    true,
				Description: "True if the duration of this lease can be extended through renewal.",
			},
			"ttl": {
				Type:        schema.TypeString,
				Optional:    true,
				Description: "User specified Time-To-Live for the STS token. Uses the Role defined default_sts_ttl when not specified",
			},
		},
	}
}

func awsAccessCredentialsDataSourceRead(d *schema.ResourceData, meta interface{}) error {
	client := meta.(*api.Client)

	backend := d.Get("backend").(string)
	credType := d.Get("type").(string)
	role := d.Get("role").(string)
	path := backend + "/" + credType + "/" + role

	arn := d.Get("role_arn").(string)
	// If the ARN is empty and only one is specified in the role definition, this should work without issue
	data := map[string][]string{
		"role_arn": {arn},
	}

	if v, ok := d.GetOk("ttl"); ok {
		data["ttl"] = []string{v.(string)}
	}

	log.Printf("[DEBUG] Reading %q from Vault with data %#v", path, data)
	secret, err := client.Logical().ReadWithData(path, data)
	if err != nil {
		return fmt.Errorf("error reading from Vault: %s", err)
	}
	log.Printf("[DEBUG] Read %q from Vault", path)

	if secret == nil {
		return fmt.Errorf("no role found at path %q", path)
	}

	d.SetId(secret.LeaseID)
	d.Set("access_key", secret.Data["access_key"])
	d.Set("secret_key", secret.Data["secret_key"])
	d.Set("security_token", secret.Data["security_token"])
	d.Set("lease_id", secret.LeaseID)
	d.Set("lease_duration", secret.LeaseDuration)
	d.Set("lease_start_time", time.Now().Format(time.RFC3339))
	d.Set("lease_renewable", secret.Renewable)

	log.Printf("[DEBUG] Waiting an additional %.f seconds for new credentials to propagate...", propagationBuffer.Seconds())
	time.Sleep(propagationBuffer)
	return nil
}
