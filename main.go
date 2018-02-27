package main

import (
	"context"
	"errors"
	"fmt"
	"log"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"time"

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
	cli, err := getKeysClient()
	onErrorFail(err, "getKeysClient failed")

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

	return err
}

func getKeysClient() (keyvault.BaseClient, error) {

	vmClient := keyvault.New()
	token, err := getKeyvaultToken()
	if err != nil {
		return vmClient, err
	}
	vmClient.Authorizer = token
	return vmClient, nil

}

func getKeyvaultToken() (authorizer autorest.Authorizer, err error) {

	oauthConfig, err := adal.NewOAuthConfig(azure.PublicCloud.ActiveDirectoryEndpoint, tenantID)
	if err != nil {
		return nil, err
	}
	updatedAuthorizeEndpoint, err := url.Parse("https://login.windows.net/" + tenantID + "/oauth2/token")
	oauthConfig.AuthorizeEndpoint = *updatedAuthorizeEndpoint
	if err != nil {
		return
	}

	cachePath := filepath.Join("cache", fmt.Sprintf("%s.token.json", clientID))

	rawToken, err := tryLoadCachedToken(cachePath)
	if err != nil {
		return nil, err
	}
	var spt *adal.ServicePrincipalToken
	if rawToken != nil && !rawToken.IsExpired() {
		defer timeTrack(time.Now(), "From File")
		spt, err = adal.NewServicePrincipalTokenFromManualToken(*oauthConfig, clientID, "https://vault.azure.net", *rawToken)
		if err != nil {
			return nil, err
		}
	} else {
		defer timeTrack(time.Now(), "No File")
		spt, err = adal.NewServicePrincipalToken(*oauthConfig, clientID, clientSecret, "https://vault.azure.net")
		if err != nil {
			return nil, err
		}
		err := spt.Refresh()
		if err != nil {
			log.Printf("Refresh token failed", err)
		}
		adRawToken := spt.Token()
		err = adal.SaveToken(cachePath, 0600, adRawToken)
		if err != nil {
			return nil, err
		}
		log.Printf("Saved token to cache. path=%q", cachePath)
	}

	authorizer = autorest.NewBearerAuthorizer(spt)
	return authorizer, nil
}

// LoadEnvVars loads environment variables.
func LoadEnvVars() error {
	err := gotenv.Load() // to allow use of .env file
	if err != nil && !strings.HasPrefix(err.Error(), "open .env:") {
		return err
	}
	return nil
}

func tryLoadCachedToken(cachePath string) (*adal.Token, error) {
	//log.Printf("Attempting to load token from cache. path=%q", cachePath)

	// Check for file not found so we can suppress the file not found error
	// LoadToken doesn't discern and returns error either way
	if _, err := os.Stat(cachePath); err != nil {
		if os.IsNotExist(err) {
			log.Printf("Cache path does not exist. Path=%q", cachePath)
			return nil, nil
		}
		return nil, err
	}

	token, err := adal.LoadToken(cachePath)
	if err != nil {
		return nil, fmt.Errorf("Failed to load token from file: %v", err)
	}
	//log.Printf("Success: loaded token from cache. path=%q", cachePath)
	return token, nil
}

func onErrorFail(err error, message string) {
	if err != nil {
		fmt.Printf("%s: %s", message, err)
		os.Exit(1)
	}
}

func timeTrack(start time.Time, name string) {
	elapsed := time.Since(start)
	log.Printf("%s took %s", name, elapsed)
}
