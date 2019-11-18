Azure Storage Volume Plugin
======================

This is a managed Docker volume plugin to allow Docker containers to access Azure Storage file shares.  The cifs-utils do not need to be installed on the host and everything is managed within the plugin.

### Caveats:

- This is a managed plugin only, no legacy support.
- There are many possible options for `mount.cifs` so rather than restricting what can be done, the plugin expects the configuration file to provide all the necessary information (except for credentials).
- In order to properly support versions use `--alias` when installing the plugin.
- It uses the same format as docker-volume-netshare for the mount points to facilitate migrations.
- **There is no robust error handling.  So garbage in -> garbage out**

## Credentials

The plugin expects `username` and `encryptedPassword` options when creating volumes. The plugin engine uses
`AZURE_KEYVAULT` and `AZURE_KEY_NAME` variables to connect to Azure Key Vault and call `decrypt` endpoint. You
need to encrypt the storage account access key yourself.

## Usage

This uses the `driver_opts.cifsopts` to define the list of options to pass to the mount command (a map couldn't be used as some options have no value and will limit future options from being added if I chose to add them.   In addition, the plugin variable `DEFAULT_CIFSOPTS` can be used to set up the default value for `driver_opts.cifsopts` if it is not specified.  For the most part my SMB shares are on Windows and so my `DEFAULT_CIFSOPTS=vers=3.02,mfsymlinks,file_mode=0666,dir_mode=0777`



Example in docker-compose.yml assuming the alias was set as `cifs`:

    volumes:
      sample:
        driver: azure-storage
        driver_opts:
          cifsopts: vers=3.02,mfsymlinks,file_mode=0666,dir_mode=0777
          username: storage-account-user
          encryptedPassword: jaiosdjfoijef90qwejf90jqeoifjqwelfqfd==
          share: //storageaccount.file.core.windows.net/myVolume
        name: "myVolume"

The values above correspond to the following mounting command:

    mount -t cifs \
      -o vers=3.02,mfsymlinks,file_mode=0666,dir_mode=0777,username=storage-account-user,password=decryptedaccesskey
      //storageaccount.file.core.windows.net/myVolume [generated_mount_point]

## Testing outside the swarm

This is an example of mounting and testing a store outside the swarm.  It is assuming the share is called `noriko/s`.

    docker plugin install evops/azure-storage-volume-plugin --alias azure-storage --grant-all-permissions AZURE_KEYVAULT=keyvault AZURE_KEY_NAME=azure-decryption-key
    
    docker volume create -d azure-storage --opt cifsopts=vers=3.02,mfsymlinks,file_mode=0666,dir_mode=0777 --opt username=azure-user --opt encryptedPassword=alsdjflasjdflsajflasjfalsfkasldf== myVolume
    docker run -it -v myVolume:/mnt alpine
