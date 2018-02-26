# Accessing Secrets in an Azure Key Vault from Go

## Azure Setup

### Azure CLI 2.x

I prefer to run my Azure CLI in a docker container:

```shell
docker run -td --name azureCli azuresdk/azure-cli-python
```

Then I alias 'az' to call the azure cli in the Docker container.

```shell
alias az="docker exec -it azureCli az"
```

Now we can use "az" just as if we'd installed the CLI on our dev box.

Next we will need to login to Azure via the CLI if we haven't before:

```shell
az login
```

### Get Subscription ID and Tenant ID

> NOTE: It might actually get a good idea to `cp env.tpl .env` now, open .env and follow along as we gather IDs and other data for our client app to use.

```shell
az account list
```

```json
[
 {
    "cloudName": "AzureCloud",
    "id": "UUID",
    "isDefault": true,
    "name": "VSE 02",
    "state": "Enabled",
    "tenantId": "UUID",
    "user": {
      "name": "fake-email@gmail.com",
      "type": "user"
    }
  }
]
```

Pick the subscription you want to use:

```shell
az account set --subscription <UUID>
```

> We will be using the Subscription ID and Tenant ID in our code later on so remember:
> AZ_SUBSCRIPTION_ID = id
> AZ_TENANT_ID = tenantId

### Register the Key Vault Provider

You only need to do this once per subscription:

```shell
az provider register -n Microsoft.KeyVault
```

### Create a resource group

```shell
az group create --name "goKeyVault" --location "centralus"
```

### Create a Key Vault

> Note that the Key Vault name needs to be globally unique since it will make up part of a URI formed as: https://yourKeyVaultName.vault.azure.net/

```shell
az keyvault create --name 'goKeyVaultTest1' --resource-group 'goKeyVault' --location 'centralus'
```

### Create Secrets

Next we will create secrets in our Key Vault:

```shell
az keyvault secret set --vault-name 'goKeyVaultTest1' --name 'UserName' --value 'gotestuser'
```

```json
{
  "attributes": {
    "created": "2018-02-25T18:32:25+00:00",
    "enabled": true,
    "expires": null,
    "notBefore": null,
    "recoveryLevel": "Purgeable",
    "updated": "2018-02-25T18:32:25+00:00"
  },
  "contentType": null,
  "id": "https://gokeyvaulttest1.vault.azure.net/secrets/UserName/aef49f415a2a4eb0ac5a21155a7cbf24",
  "kid": null,
  "managed": null,
  "tags": {
    "file-encoding": "utf-8"
  },
  "value": "gotestuser"
}
```

```shell
az keyvault secret set --vault-name 'goKeyVaultTest1' --name 'Password' --value 'correcthorsebatterystaple'
```

```json
{
  "attributes": {
    "created": "2018-02-25T18:33:05+00:00",
    "enabled": true,
    "expires": null,
    "notBefore": null,
    "recoveryLevel": "Purgeable",
    "updated": "2018-02-25T18:33:05+00:00"
  },
  "contentType": null,
  "id": "https://gokeyvaulttest1.vault.azure.net/secrets/Password/8142a26d3a02425282da3da565f4a952",
  "kid": null,
  "managed": null,
  "tags": {
    "file-encoding": "utf-8"
  },
  "value": "correcthorsebatterystaple"
}
```

> Once again we will need some information from the resultant JSON for our code a little later on. For now just capture the full id (URL):

https://gokeyvaulttest1.vault.azure.net/secrets/UserName/aef49f415a2a4eb0ac5a21155a7cbf24
https://gokeyvaulttest1.vault.azure.net/secrets/Password/8142a26d3a02425282da3da565f4a952

{---------------base url---------------}secrets{--name--}{-----------version------------}

> and yes these IDs have been killed with fire!

For reasons that will become clear later, we will add the password again.

```shell
az keyvault secret set --vault-name 'goKeyVaultTest1' --name 'Password' --value 'thisisthelatestpasswordwithnohorseorbattery'
```

> Don't worry about capturing the id/URL of this one.

### List Secrets

We can list the secrets we have in the Vault:

```shell
az keyvault secret list --vault-name 'goKeyVaultTest1'
```

### Create a Service Principal

We will create a Service Principal that will have access to our Key Vault. You really don't want your code accessing the Vault as you.

```shell
az ad sp create-for-rbac -n "goKeyVaultTest"
```

```json
{
  "appId": "UUID",
  "displayName": "goKeyVaultTest",
  "name": "http://goKeyVaultTest",
  "password": "UUID",
  "tenant": "UUID"
}
```

> We will need some information from the resultant JSON for our code - make sure to note the appId <UUID> and the password from the resulting JSON. In our .env file... 
> AZ_CLIENT_ID = appID
> AZ_CLIENT_SECRET = password

### Give the SP rights to our Key Vault

Now we give the Service Principal rights to our Key Vault. Only read (get) and list, though.

```shell
az keyvault set-policy --name 'goKeyVaultTest1' --spn UUID --secret-permissions get list
```

## Go Time

Finally! Now we should have everything setup to work with our sample code.

```shell
cd $GOPATH/src
git clone https://github.com/stevebargelt/goAzureKeyVault.git
cd goAzureKeyVault
```

### Edit .env

We've gathered all of the necessary IDs for our environment variables.

```shell
cp env.tpl .env
```

Fill in the vars:

```shell
AZ_SUBSCRIPTION_ID= #Your Azure subscription ID
AZ_TENANT_ID= # Azure tenant ID
AZ_CLIENT_ID= # Service Principal appID (from JSON response)
AZ_CLIENT_SECRET= # Service Principal password (from JSON response)
VAULT_BASE_URL=https://gokeyvaulttest1.vault.azure.net #(from JSON response)
USER_SECRET_NAME=UserName
USER_SECRET_VERSION=aef49f415a2a4eb0ac5a21155a7cbf24 # (from JSON response)
PASSWORD_SECRET_NAME=Password
PASSWORD_SECRET_VERSION=8142a26d3a02425282da3da565f4a952 # (from JSON response)
```

Once these changes are saved.

```shell
dep ensure
go build main.go
go run main.go
```

The result:

```text
Getting Key Vault
Username Value= gotestuser
--- With Version Set ---
Password Value= correcthorsebatterystaple
--- Current ---
Password Value= thisisthelatestpasswordwithnohorseorbattery
```

### Cleanup

We can clean up this test. But please be CAREFUL this removes the Resource Group and every single resource under it.

```shell
az group delete --name goKeyVault --yes --no-wait
```

Remove the Service Principal

```shell
az ad sp delete --id <UUID>
```