package main

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"net/http/httptest"
	"net/http/httputil"
	"strings"
	"testing"

	mountedvolume "../mounted-volume"
	"github.com/docker/go-plugins-helpers/volume"
)

func TestDecryption(t *testing.T) {
}

func TestMountOptions(t *testing.T) {
	metadataResponse := AzureMetadataResponse{
		AccessToken: "123",
	}

	testServer := httptest.NewServer(http.HandlerFunc(func(res http.ResponseWriter, req *http.Request) {
		log.Println("Request for", req.URL.Path)

		if req.URL.Path == "/encryptDecrypt/decrypt" {
			response := AzureEncryptDecryptResponse{
				Value: "U2VjcmV0IFBhc3N3b3Jk",
			}

			res.WriteHeader(200)
			res.Header().Add("Content-Type", "application/json")
			bytes, _ := json.Marshal(response)

			res.Write(bytes)
		}

		if req.URL.Path == "/keyvault" {
			keyResponse := AzureKeyResponse{
				Key: AzureKey{
					Kid: fmt.Sprintf("http://%s/encryptDecrypt", req.Host),
				},
			}

			res.WriteHeader(200)
			res.Header().Add("Content-Type", "application/json")
			bytes, _ := json.Marshal(keyResponse)

			res.Write(bytes)
		}

		if req.URL.Path == "/" {
			res.WriteHeader(200)
			res.Header().Add("Content-Type", "application/json")
			bytes, _ := json.Marshal(metadataResponse)

			res.Write(bytes)
		}
	}))
	defer func() { testServer.Close() }()

	d := &azureStorageDriver{
		Driver:           *mountedvolume.NewDriver("cifs", true, "azure-storage-test", "local"),
		azureKeyName:     "secretKey",
		azureKeyVault:    "azureKeyVault",
		azureMetadataURL: testServer.URL,
		azureKeyVaultURL: fmt.Sprintf("%s/keyvault?keyvault=%%s&keyname=%%s", testServer.URL),
		debug:            true,
	}

	log.Println(d)

	defer d.Close()
	d.Init(d)

	options := make(map[string]string)
	options["share"] = "//share-to-mount.azure"
	options["username"] = "secret-username"
	options["encryptedPassword"] = "secret-password"
	options["cifsopts"] = "vers=3.0,file_mode=777"
	req := &volume.CreateRequest{
		Name:    "NamedVolume",
		Options: options,
	}

	mountOptions := d.MountOptions(req)
	log.Println(mountOptions)
	for _, n := range mountOptions {
		if strings.Contains(n, "password") && !strings.Contains(n, "Secret Password") {
			t.FailNow()
		}
	}

	d.Driver.Create(req)

	volumeGetRequest := &volume.GetRequest{
		Name: "NamedVolume",
	}
	volumeGetResponse, _ := d.Driver.Get(volumeGetRequest)
	log.Println(volumeGetResponse.Volume.Status)
}
