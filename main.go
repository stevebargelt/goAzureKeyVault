package main

import (
	"context"
	"errors"
	"fmt"
	"log"
	"net/url"
	"os"
	"strings"

	"github.com/Azure/azure-sdk-for-go/services/keyvault/2016-10-01/keyvault"
	"github.com/Azure/go-autorest/autorest"
	"github.com/Azure/go-autorest/autorest/adal"
	"github.com/Azure/go-autorest/autorest/azure"
	"github.com/subosito/gotenv"
)

var (
	//Define these in .env
	vaultBaseURL          string
	userSecretName        string
	userSecretVersion     string
	passwordSecretName    string
	passwordSecretVersion string
	subscriptionID        string
	tenantID              string
	clientID              string
	clientSecret          string

	oauthConfig *adal.OAuthConfig
)

func main() {

	err := parseArgs()
	if err != nil {
		log.Fatalf("failed to parse args: %s\n", err)
	}

	fmt.Println("Getting Key Vault")
	cli := getKeysClient()

	username, err := getSecret(&cli, vaultBaseURL, userSecretName, userSecretVersion)
	onErrorFail(err, "getUsername failed")
	fmt.Printf("Username Value= %s\n", username)

	//If we omit the secret version we get the current (latest) secret
	fmt.Println("--- Password with no version set (current) ---")
	password, err := getSecret(&cli, vaultBaseURL, passwordSecretName, "")
	onErrorFail(err, "getPassword failed")
	fmt.Printf("  Password Value= %s\n", password)

	//Using the secret version we can access specific versions of the secret (older, etc.)
	fmt.Printf("--- Password version %s ---\n", passwordSecretVersion)
	password, err = getSecret(&cli, vaultBaseURL, passwordSecretName, passwordSecretVersion)
	onErrorFail(err, "getPassword failed")
	fmt.Printf("  Password Value= %s\n", password)
}

func getSecret(cli *keyvault.BaseClient, vaultBaseURL string, secretName string, secretVersion string) (string, error) {

	ctx := context.Background()
	secretBundle, err := cli.GetSecret(ctx, vaultBaseURL, secretName, secretVersion)
	if err != nil {
		return "", err
	}

	return *secretBundle.Value, nil
}

func parseArgs() error {

	err := LoadEnvVars()
	if err != nil {
		return err
	}

	var message string
	vaultBaseURL = os.Getenv("VAULT_BASE_URL")
	if vaultBaseURL == "" {
		message += fmt.Sprintln("VAULT_BASE_URL missing")
	}
	userSecretName = os.Getenv("USER_SECRET_NAME")
	if userSecretName == "" {
		message += fmt.Sprintln("USER_SECRET_NAME missing")
	}
	userSecretVersion = os.Getenv("USER_SECRET_VERSION")
	if userSecretVersion == "" {
		message += fmt.Sprintln("USER_SECRET_VERSION missing")
	}
	passwordSecretName = os.Getenv("PASSWORD_SECRET_NAME")
	if userSecretName == "" {
		message += fmt.Sprintln("PASSWORD_SECRET_NAME missing")
	}
	passwordSecretVersion = os.Getenv("PASSWORD_SECRET_VERSION")
	if passwordSecretVersion == "" {
		message += fmt.Sprintln("PASSWORD_SECRET_VERSION missing")
	}
	tenantID = os.Getenv("AZ_TENANT_ID")
	if tenantID == "" {
		message += fmt.Sprintln("AZ_TENANT_ID missing")
	}
	clientID = os.Getenv("AZ_CLIENT_ID")
	if clientID == "" {
		message += fmt.Sprintln("AZ_CLIENT_ID missing")
	}
	clientSecret = os.Getenv("AZ_CLIENT_SECRET")
	if clientSecret == "" {
		message += fmt.Sprintln("AZ_CLIENT_SECRET missing")
	}

	if len(message) > 0 {
		message += "| need to be defined in .env or environment variable."
		return errors.New(message)
	}

	oauthConfig, err = adal.NewOAuthConfig(azure.PublicCloud.ActiveDirectoryEndpoint, tenantID)

	return err
}

func getKeysClient() keyvault.BaseClient {
	token, _ := getKeyvaultToken()
	vmClient := keyvault.New()
	vmClient.Authorizer = token
	//vmClient.AddToUserAgent("goAzureKeyVault") //Where does this show?
	return vmClient
}

func getKeyvaultToken() (authorizer autorest.Authorizer, err error) {
	config, err := adal.NewOAuthConfig(azure.PublicCloud.ActiveDirectoryEndpoint, tenantID)
	updatedAuthorizeEndpoint, err := url.Parse("https://login.windows.net/" + tenantID + "/oauth2/token")
	config.AuthorizeEndpoint = *updatedAuthorizeEndpoint
	if err != nil {
		return
	}

	spt, err := adal.NewServicePrincipalToken(
		*config,
		clientID,
		clientSecret,
		"https://vault.azure.net")

	if err != nil {
		return authorizer, err
	}
	authorizer = autorest.NewBearerAuthorizer(spt)

	return
}

// LoadEnvVars loads environment variables.
func LoadEnvVars() error {
	err := gotenv.Load() // to allow use of .env file
	if err != nil && !strings.HasPrefix(err.Error(), "open .env:") {
		return err
	}
	return nil
}

func onErrorFail(err error, message string) {
	if err != nil {
		fmt.Printf("%s: %s", message, err)
		os.Exit(1)
	}
}
