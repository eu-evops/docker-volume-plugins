package main

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"net/http/httputil"
	"os"
	"strconv"
	"strings"

	mountedvolume "../mounted-volume"
	"github.com/docker/go-plugins-helpers/volume"
)

type azureStorageDriver struct {
	credentialPath   string
	defaultCifsopts  string
	azureMetadataURL string
	azureKeyVaultURL string
	azureKeyVault    string
	azureKeyName     string
	debug            bool
	mountedvolume.Driver
}

func (p *azureStorageDriver) Validate(req *volume.CreateRequest) error {
	return nil
}

func (p *azureStorageDriver) PreMount(req *volume.MountRequest) error {
	return nil
}

func (p *azureStorageDriver) PostMount(req *volume.MountRequest) {
}

// AzureMetadataResponse Model for Azure Metadata response
type AzureMetadataResponse struct {
	AccessToken  string `json:"access_token"`
	RefreshToken string `json:"refresh_token"`
	ExpiresIn    string `json:"expires_in"`
	ExpiresOn    string `json:"expires_on"`
	NotBefore    string `json:"not_before"`
	Resource     string `json:"resource"`
	TokenType    string `json:"token_type"`
}

// AzureKeyVaultEncryptDecryptBody model for encrypt/decrypt request body
type AzureKeyVaultEncryptDecryptBody struct {
	Algorithm string `json:"alg"`
	Value     string `json:"value"`
}

// AzureKey model for AzureKey
type AzureKey struct {
	Kid string `json:"kid"`
}

// AzureKeyResponse model for AzureKey response
type AzureKeyResponse struct {
	Key AzureKey `json:"key"`
}

type AzureEncryptDecryptResponse struct {
	Value string `json:"value"`
}

func (p *azureStorageDriver) decryptPasswordUsingAzureKeyVault(encryptedPassword string) (string, error) {
	log.Printf("Reading key from Azure KV: %s\n", p.azureKeyVault)
	log.Printf("Decrypting from Azure using key: %s\n", p.azureKeyName)

	// 1. Obtain Azure Storage access token from machine identity # https://docs.microsoft.com/en-us/azure/active-directory/managed-identities-azure-resources/how-to-use-vm-token
	//	http://169.254.169.254/metadata/identity/oauth2/token?api-version=2018-02-01&resource=https://management.azure.com/ HTTP/1.1 Metadata:true
	// 2. Obtain Azure Key URL
	// 3. Call decrypt endpoint
	// 4. Adjust Base64 due to a bug in azure service: https://blog.ahasayen.com/how-to-use-azure-key-vault-with-powershell-to-encrypt-data/

	httpClient := http.Client{}
	metadataURL := p.azureMetadataURL

	log.Println("Requesting VM metadata from url", metadataURL)
	metadataRequest, _ := http.NewRequest("GET", metadataURL, nil)
	metadataRequest.Header.Add("Metadata", "true")
	metadataResponse, _ := httpClient.Do(metadataRequest)

	if p.debug {
		dump, _ := httputil.DumpResponse(metadataResponse, true)
		log.Println("Key response", string(dump))
	}

	var metadata AzureMetadataResponse
	if err := json.NewDecoder(metadataResponse.Body).Decode(&metadata); err != nil {
		log.Println("Error decoding Azure Metadata Response")
		return "", err
	}

	log.Println("Getting key information from", fmt.Sprintf(p.azureKeyVaultURL, p.azureKeyVault, p.azureKeyName))
	keyRequest, _ := http.NewRequest("GET", fmt.Sprintf(p.azureKeyVaultURL, p.azureKeyVault, p.azureKeyName), nil)
	keyRequest.Header.Add("Authorization", fmt.Sprintf("Bearer %s", metadata.AccessToken))
	keyResponse, err := httpClient.Do(keyRequest)
	if err != nil {
		log.Println("Error requesting key information", err)
		return "", err
	}

	if p.debug {
		dump, _ := httputil.DumpResponse(keyResponse, true)
		log.Println("Key response", string(dump))
	}

	var key AzureKeyResponse
	if err := json.NewDecoder(keyResponse.Body).Decode(&key); err != nil {
		log.Println("Error obtaining key information from Azure", err)
		return "", err
	}

	log.Println("Azure key response", key)

	// Key URL form Azure
	keyURL := key.Key.Kid

	decryptRequestBody := AzureKeyVaultEncryptDecryptBody{
		Algorithm: "RSA-OAEP",
		Value:     encryptedPassword,
	}

	decryptJSONBody, _ := json.Marshal(decryptRequestBody)

	log.Println("Decrypting password with url", keyURL)
	decryptRequest, _ := http.NewRequest(http.MethodPost, fmt.Sprintf("%s/decrypt?api-version=2016-10-01", keyURL), bytes.NewBuffer(decryptJSONBody))
	decryptRequest.Header.Add("Content-Type", "application/json")
	decryptRequest.Header.Add("Authorization", fmt.Sprintf("Bearer %s", metadata.AccessToken))

	decryptResponse, _ := httpClient.Do(decryptRequest)

	if p.debug {
		dump, _ := httputil.DumpResponse(decryptResponse, true)
		log.Println("Decrypt response", string(dump))
	}

	var decryptionResult AzureEncryptDecryptResponse
	if err := json.NewDecoder(decryptResponse.Body).Decode(&decryptionResult); err != nil {
		log.Println("Error decoding decryption response", err)
		return "", err
	}

	base64Password := decryptionResult.Value
	numberOfMissingCharacters := len(base64Password) % 4
	
	for i := 0; i < numberOfMissingCharacters; i++ {
		base64Password += "="
	}

	decodedString, err := base64.StdEncoding.DecodeString(base64Password)

	if err != nil {
		return "", err
	}

	return string(decodedString), nil
}

func (p *azureStorageDriver) MountOptions(req *volume.CreateRequest) []string {
	cifsopts, cifsoptsInOpts := req.Options["cifsopts"]
	share := req.Options["share"]
	username := req.Options["username"]
	encryptedPassword := req.Options["encryptedPassword"]

	var cifsoptsArray []string
	if cifsoptsInOpts {
		cifsoptsArray = append(cifsoptsArray, strings.Split(cifsopts, ",")...)
	} else {
		cifsoptsArray = append(cifsoptsArray, strings.Split(p.defaultCifsopts, ",")...)
	}

	if encryptedPassword != "" {
		password, err := p.decryptPasswordUsingAzureKeyVault(encryptedPassword)
		if err != nil {
			log.Panicln("Failed to decrypt password", err)
		}
		cifsoptsArray = append(cifsoptsArray, fmt.Sprintf("password=%s", password))
	}

	if username != "" {
		cifsoptsArray = append(cifsoptsArray, fmt.Sprintf("username=%s", username))
	}

	log.Printf("%+v\n", cifsoptsArray)

	return []string{"-t", "cifs", "-o", strings.Join(cifsoptsArray, ","), share}
}

func buildDriver() *azureStorageDriver {
	credentialPath := os.Getenv("CREDENTIAL_PATH")
	defaultCifsopts := os.Getenv("DEFAULT_CIFSOPTS")
	azureKeyVault := os.Getenv("AZURE_KEYVAULT")
	azureSecretName := os.Getenv("AZURE_KEY_NAME")
	azureMetadataURL := os.Getenv("AZURE_METADATA_URL")
	azureKeyVaultURL := os.Getenv("AZURE_KEYVAULT_URL")
	debug, _ := strconv.ParseBool(os.Getenv("DEBUG"))

	if _, err := os.Stat(volume.DefaultDockerRootDirectory); os.IsNotExist(err) {
		log.Printf("Creating docker default directory: %s\n", volume.DefaultDockerRootDirectory)
		os.MkdirAll(volume.DefaultDockerRootDirectory, os.FileMode(0755))
	}

	d := &azureStorageDriver{
		Driver:           *mountedvolume.NewDriver("mount", true, "azure-storage", "local"),
		credentialPath:   credentialPath,
		defaultCifsopts:  defaultCifsopts,
		azureKeyVault:    azureKeyVault,
		azureKeyName:     azureSecretName,
		azureMetadataURL: azureMetadataURL,
		azureKeyVaultURL: azureKeyVaultURL,
		debug:            debug,
	}
	d.Init(d)
	return d
}

func main() {
	log.SetFlags(0)
	d := buildDriver()
	defer d.Close()
	d.ServeUnix()
}
