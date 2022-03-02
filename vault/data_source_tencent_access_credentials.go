package vault

import (
	"fmt"
	"log"
	"time"

	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
	"github.com/hashicorp/vault/api"
	"github.com/tencentcloud/tencentcloud-sdk-go/tencentcloud/common"
	"github.com/tencentcloud/tencentcloud-sdk-go/tencentcloud/common/profile"
	sts "github.com/tencentcloud/tencentcloud-sdk-go/tencentcloud/sts/v20180813"
)

const (
	// sequentialSuccessesRequired is the number of times the test of an eventually consistent
	// credential must succeed before we return it for use.
	sequentialSuccessesRequired = 5

	// sequentialSuccessTimeLimit is how long we'll wait for eventually consistent Tencent creds
	// to propagate before giving up. In real life, we've seen it take up to 15 seconds, so
	// this is ample and if it's unsuccessful there's something else wrong.
	sequentialSuccessTimeLimit = time.Minute

	// retryTimeOut is how long we'll wait before timing out when we're retrying credentials.
	// This corresponds to Vault's default 30-second request timeout.
	retryTimeOut = 30 * time.Second

	// propagationBuffer is the added buffer of time we'll wait after N sequential successes
	// before returning credentials for use.
	propagationBuffer = 3 * time.Second
)

func tencentAccessCredentialsDataSource() *schema.Resource {
	return &schema.Resource{
		Read: tencentAccessCredentialsDataSourceRead,

		Schema: map[string]*schema.Schema{
			"backend": {
				Type:        schema.TypeString,
				Required:    true,
				Description: "Tencent Secret Backend to read credentials from.",
			},
			"role": {
				Type:        schema.TypeString,
				Required:    true,
				Description: "Tencent Secret Role to read credentials from.",
			},
			"sts_region": {
				Type:        schema.TypeString,
				Optional:    true,
				Description: "TencentCloud Region for use Credential Validation",
				Default:     "ap-guangzhou",
			},

			"access_key": {
				Type:        schema.TypeString,
				Computed:    true,
				Description: "Tencent access key ID read from Vault.",
			},
			"secret_key": {
				Type:        schema.TypeString,
				Computed:    true,
				Description: "Tencent secret key read from Vault.",
			},
			"security_token": {
				Type:        schema.TypeString,
				Computed:    true,
				Description: "Tencent security token read from Vault. (Only returned if type is 'sts').",
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
			"arn": {
				Type:        schema.TypeString,
				Computed:    true,
				Description: "TencentCloud CAM Identity ARN",
			},
		},
	}
}

func tencentAccessCredentialsDataSourceRead(d *schema.ResourceData, meta interface{}) error {
	client := meta.(*api.Client)

	backend := d.Get("backend").(string)
	role := d.Get("role").(string)
	path := backend + "/creds/" + role

	data := map[string][]string{}

	log.Printf("[DEBUG] Reading %q from Vault with data %#v", path, data)
	secret, err := client.Logical().ReadWithData(path, data)
	if err != nil {
		return fmt.Errorf("error reading from Vault: %s", err)
	}
	log.Printf("[DEBUG] Read %q from Vault", path)

	if secret == nil {
		return fmt.Errorf("no role found at path %q", path)
	}

	log.Printf("[DEBUG] Waiting an additional %.f seconds for new credentials to propagate...", propagationBuffer.Seconds())
	time.Sleep(propagationBuffer)

	accessKey := secret.Data["secret_id"].(string)
	secretKey := secret.Data["secret_key"].(string)
	securityToken := secret.Data["token"].(string)

	cred := common.NewTokenCredential(accessKey, secretKey, securityToken)
	stsClient, err := sts.NewClient(cred, d.Get("sts_region").(string), profile.NewClientProfile())
	if err != nil {
		return fmt.Errorf("error create TencentCloud STS Client %s", err.Error())
	}
	stsRes, err := stsClient.GetCallerIdentity(sts.NewGetCallerIdentityRequest())
	if err != nil {
		return fmt.Errorf("error request GetCallerIdentity %s", err.Error())
	}
	log.Printf("[DEBUG] TencentCloud STS GetCallerIdentity Success arn=%s", *stsRes.Response.Arn)

	d.SetId(secret.LeaseID)
	d.Set("access_key", accessKey)
	d.Set("secret_key", secretKey)
	d.Set("security_token", securityToken)
	d.Set("lease_id", secret.LeaseID)
	d.Set("lease_duration", secret.LeaseDuration)
	d.Set("lease_start_time", time.Now().Format(time.RFC3339))
	d.Set("lease_renewable", secret.Renewable)
	d.Set("arn", *stsRes.Response.Arn)
	return nil
}
