# Runvoy CLI Documentation

This document contains all available CLI commands, their descriptions, flags, and examples.
## runvoy

runvoy

runvoy - 0.0.0-development
Isolated, repeatable execution environments for your commands

### Options

```
      --debug            Enable debugging logs
  -h, --help             help for runvoy
      --timeout string   Timeout for command execution (e.g., 10m, 30s, 1h) (default "10m")
      --verbose          Verbose output
```

### SEE ALSO

* [runvoy claim](runvoy_claim.md)	 - Claim a user's API key
* [runvoy configure](runvoy_configure.md)	 - Configure local environment with API key and endpoint URL
* [runvoy images](runvoy_images.md)	 - Docker images management commands
* [runvoy kill](runvoy_kill.md)	 - Kill a running command execution
* [runvoy list](runvoy_list.md)	 - List executions
* [runvoy logs](runvoy_logs.md)	 - Get logs for an execution
* [runvoy playbook](runvoy_playbook.md)	 - Manage and execute playbooks
* [runvoy run](runvoy_run.md)	 - Run a command
* [runvoy secrets](runvoy_secrets.md)	 - Secrets management commands
* [runvoy status](runvoy_status.md)	 - Get the status of a command execution
* [runvoy users](runvoy_users.md)	 - User management commands
* [runvoy version](runvoy_version.md)	 - Show the version of the CLI



### runvoy claim

Claim a user's API key

Claim a user's API key using the given token

**Examples:**

```bash
  - runvoy claim 1234567890
```

### Options

```
  -h, --help   help for claim
```

### Options inherited from parent commands

```
      --debug            Enable debugging logs
      --timeout string   Timeout for command execution (e.g., 10m, 30s, 1h) (default "10m")
      --verbose          Verbose output
```

### SEE ALSO

* [runvoy](runvoy.md)	 - runvoy



### runvoy configure

Configure local environment with API key and endpoint URL

Configure the local environment with your API key and endpoint URL.
This creates or updates the configuration file at [1m~/.runvoy/config.yaml[22m

### Options

```
  -h, --help   help for configure
```

### Options inherited from parent commands

```
      --debug            Enable debugging logs
      --timeout string   Timeout for command execution (e.g., 10m, 30s, 1h) (default "10m")
      --verbose          Verbose output
```

### SEE ALSO

* [runvoy](runvoy.md)	 - runvoy



### runvoy images

Docker images management commands

### Options

```
  -h, --help   help for images
```

### Options inherited from parent commands

```
      --debug            Enable debugging logs
      --timeout string   Timeout for command execution (e.g., 10m, 30s, 1h) (default "10m")
      --verbose          Verbose output
```

### SEE ALSO

* [runvoy](runvoy.md)	 - runvoy
* [runvoy images list](runvoy_images_list.md)	 - List all registered Docker images
* [runvoy images register](runvoy_images_register.md)	 - Register a new Docker image
* [runvoy images show](runvoy_images_show.md)	 - Show detailed information about a Docker image
* [runvoy images unregister](runvoy_images_unregister.md)	 - Unregister a Docker image



#### runvoy images list

List all registered Docker images

### Options

```
  -h, --help   help for list
```

### Options inherited from parent commands

```
      --debug            Enable debugging logs
      --timeout string   Timeout for command execution (e.g., 10m, 30s, 1h) (default "10m")
      --verbose          Verbose output
```

### SEE ALSO

* [runvoy images](runvoy_images.md)	 - Docker images management commands



#### runvoy images register

Register a new Docker image

**Examples:**

```bash
  - runvoy images register alpine:latest
  - runvoy images register ecr-public.us-east-1.amazonaws.com/docker/library/ubuntu:22.04
  - runvoy images register ubuntu:22.04 --set-default
```

### Options

```
      --cpu string                Optional CPU value (e.g., 256, 1024). Defaults to 256 if not specified
  -h, --help                      help for register
      --memory string             Optional Memory value (e.g., 512, 2048). Defaults to 512 if not specified
      --runtime-platform string   Optional runtime platform (e.g., Linux/ARM64, Linux/X86_64). Defaults to Linux/ARM64 if not specified
      --set-default               Set this image as the default image
      --task-exec-role string     Optional task execution role name for the image
      --task-role string          Optional task role name for the image
```

### Options inherited from parent commands

```
      --debug            Enable debugging logs
      --timeout string   Timeout for command execution (e.g., 10m, 30s, 1h) (default "10m")
      --verbose          Verbose output
```

### SEE ALSO

* [runvoy images](runvoy_images.md)	 - Docker images management commands



#### runvoy images show

Show detailed information about a Docker image

**Examples:**

```bash
  - runvoy images show alpine:latest
  - runvoy images show alpine:latest-a1b2c3d4
```

### Options

```
  -h, --help   help for show
```

### Options inherited from parent commands

```
      --debug            Enable debugging logs
      --timeout string   Timeout for command execution (e.g., 10m, 30s, 1h) (default "10m")
      --verbose          Verbose output
```

### SEE ALSO

* [runvoy images](runvoy_images.md)	 - Docker images management commands



#### runvoy images unregister

Unregister a Docker image

**Examples:**

```bash
  - runvoy images unregister alpine:latest
```

### Options

```
  -h, --help   help for unregister
```

### Options inherited from parent commands

```
      --debug            Enable debugging logs
      --timeout string   Timeout for command execution (e.g., 10m, 30s, 1h) (default "10m")
      --verbose          Verbose output
```

### SEE ALSO

* [runvoy images](runvoy_images.md)	 - Docker images management commands



### runvoy kill

Kill a running command execution

### Options

```
  -h, --help   help for kill
```

### Options inherited from parent commands

```
      --debug            Enable debugging logs
      --timeout string   Timeout for command execution (e.g., 10m, 30s, 1h) (default "10m")
      --verbose          Verbose output
```

### SEE ALSO

* [runvoy](runvoy.md)	 - runvoy



### runvoy list

List executions

List all executions present in the runvoy backend

### Options

```
  -h, --help   help for list
```

### Options inherited from parent commands

```
      --debug            Enable debugging logs
      --timeout string   Timeout for command execution (e.g., 10m, 30s, 1h) (default "10m")
      --verbose          Verbose output
```

### SEE ALSO

* [runvoy](runvoy.md)	 - runvoy



### runvoy logs

Get logs for an execution

### Options

```
  -h, --help   help for logs
```

### Options inherited from parent commands

```
      --debug            Enable debugging logs
      --timeout string   Timeout for command execution (e.g., 10m, 30s, 1h) (default "10m")
      --verbose          Verbose output
```

### SEE ALSO

* [runvoy](runvoy.md)	 - runvoy



### runvoy playbook

Manage and execute playbooks

Manage and execute reusable command execution configurations defined in YAML files

### Options

```
  -h, --help   help for playbook
```

### Options inherited from parent commands

```
      --debug            Enable debugging logs
      --timeout string   Timeout for command execution (e.g., 10m, 30s, 1h) (default "10m")
      --verbose          Verbose output
```

### SEE ALSO

* [runvoy](runvoy.md)	 - runvoy
* [runvoy playbook list](runvoy_playbook_list.md)	 - List all available playbooks
* [runvoy playbook run](runvoy_playbook_run.md)	 - Execute a playbook
* [runvoy playbook show](runvoy_playbook_show.md)	 - Show playbook details



#### runvoy playbook list

List all available playbooks

List all playbooks found in the .runvoy directory

### Options

```
  -h, --help   help for list
```

### Options inherited from parent commands

```
      --debug            Enable debugging logs
      --timeout string   Timeout for command execution (e.g., 10m, 30s, 1h) (default "10m")
      --verbose          Verbose output
```

### SEE ALSO

* [runvoy playbook](runvoy_playbook.md)	 - Manage and execute playbooks



#### runvoy playbook run

Execute a playbook

Execute a playbook with optional flag overrides

### Options

```
  -p, --git-path string   Override git path
  -r, --git-ref string    Override git reference
  -g, --git-repo string   Override git repository URL
  -h, --help              help for run
  -i, --image string      Override image
      --secret strings    Add additional secrets (merge with playbook secrets)
```

### Options inherited from parent commands

```
      --debug            Enable debugging logs
      --timeout string   Timeout for command execution (e.g., 10m, 30s, 1h) (default "10m")
      --verbose          Verbose output
```

### SEE ALSO

* [runvoy playbook](runvoy_playbook.md)	 - Manage and execute playbooks



#### runvoy playbook show

Show playbook details

Display the full content of a playbook

### Options

```
  -h, --help   help for show
```

### Options inherited from parent commands

```
      --debug            Enable debugging logs
      --timeout string   Timeout for command execution (e.g., 10m, 30s, 1h) (default "10m")
      --verbose          Verbose output
```

### SEE ALSO

* [runvoy playbook](runvoy_playbook.md)	 - Manage and execute playbooks



### runvoy run

Run a command

Run a command in a remote environment with optional Git repository cloning.

User environment variables prefixed with RUNVOY_USER_ are saved to .env file
in the command working directory.

**Examples:**

```bash
  - runvoy run echo hello world
  - runvoy run terraform plan

  # With private Git repository cloning
  - runvoy run --secret github-token \
               --git-repo https://github.com/mycompany/myproject.git \
               npm run test

  # With public Git repository cloning and a specific Git reference and path
  - runvoy run --git-repo https://github.com/ansible/ansible-examples.git \
               --git-ref main \
               --git-path ansible-examples/playbooks/hello_world \
               ansible-playbook site.yml

  # With user environment variables
  - RUNVOY_USER_MY_VAR=1234567890 runvoy run cat .env # Outputs => MY_VAR=1234567890

```

### Options

```
  -p, --git-path string   Git path
  -r, --git-ref string    Git reference
  -g, --git-repo string   Git repository URL
  -h, --help              help for run
  -i, --image string      Image to use
      --secret strings    Secret name to inject (repeatable)
```

### Options inherited from parent commands

```
      --debug            Enable debugging logs
      --timeout string   Timeout for command execution (e.g., 10m, 30s, 1h) (default "10m")
      --verbose          Verbose output
```

### SEE ALSO

* [runvoy](runvoy.md)	 - runvoy



### runvoy secrets

Secrets management commands

### Options

```
  -h, --help   help for secrets
```

### Options inherited from parent commands

```
      --debug            Enable debugging logs
      --timeout string   Timeout for command execution (e.g., 10m, 30s, 1h) (default "10m")
      --verbose          Verbose output
```

### SEE ALSO

* [runvoy](runvoy.md)	 - runvoy
* [runvoy secrets create](runvoy_secrets_create.md)	 - Create a new secret
* [runvoy secrets delete](runvoy_secrets_delete.md)	 - Delete a secret
* [runvoy secrets get](runvoy_secrets_get.md)	 - Get a secret by name
* [runvoy secrets list](runvoy_secrets_list.md)	 - List all secrets
* [runvoy secrets update](runvoy_secrets_update.md)	 - Update a secret



#### runvoy secrets create

Create a new secret

Create a new secret with the given name, key name (environment variable name), and value

**Examples:**

```bash
  - runvoy secrets create github-token GITHUB_TOKEN "ghp_xxxxx"
  - runvoy secrets create db-password DB_PASSWORD "secret123" --description "Database password"
```

### Options

```
      --description string   Description for the secret
  -h, --help                 help for create
```

### Options inherited from parent commands

```
      --debug            Enable debugging logs
      --timeout string   Timeout for command execution (e.g., 10m, 30s, 1h) (default "10m")
      --verbose          Verbose output
```

### SEE ALSO

* [runvoy secrets](runvoy_secrets.md)	 - Secrets management commands



#### runvoy secrets delete

Delete a secret

Delete a secret by its name

**Examples:**

```bash
  - runvoy secrets delete github-token
```

### Options

```
  -h, --help   help for delete
```

### Options inherited from parent commands

```
      --debug            Enable debugging logs
      --timeout string   Timeout for command execution (e.g., 10m, 30s, 1h) (default "10m")
      --verbose          Verbose output
```

### SEE ALSO

* [runvoy secrets](runvoy_secrets.md)	 - Secrets management commands



#### runvoy secrets get

Get a secret by name

Retrieve a secret by its name, including its value

**Examples:**

```bash
  - runvoy secrets get github-token
```

### Options

```
  -h, --help   help for get
```

### Options inherited from parent commands

```
      --debug            Enable debugging logs
      --timeout string   Timeout for command execution (e.g., 10m, 30s, 1h) (default "10m")
      --verbose          Verbose output
```

### SEE ALSO

* [runvoy secrets](runvoy_secrets.md)	 - Secrets management commands



#### runvoy secrets list

List all secrets

List all secrets in the system with their basic information

**Examples:**

```bash
  - runvoy secrets list
```

### Options

```
  -h, --help   help for list
```

### Options inherited from parent commands

```
      --debug            Enable debugging logs
      --timeout string   Timeout for command execution (e.g., 10m, 30s, 1h) (default "10m")
      --verbose          Verbose output
```

### SEE ALSO

* [runvoy secrets](runvoy_secrets.md)	 - Secrets management commands



#### runvoy secrets update

Update a secret

Update a secret's metadata (description, key_name) and/or value

**Examples:**

```bash
  - runvoy secrets update github-token --key-name GITHUB_API_TOKEN --value "new-token"
  - runvoy secrets update db-password --description "Updated database password"
```

### Options

```
      --description string   Description for the secret
  -h, --help                 help for update
      --key-name string      Environment variable name (e.g., GITHUB_TOKEN)
      --value string         Secret value to update
```

### Options inherited from parent commands

```
      --debug            Enable debugging logs
      --timeout string   Timeout for command execution (e.g., 10m, 30s, 1h) (default "10m")
      --verbose          Verbose output
```

### SEE ALSO

* [runvoy secrets](runvoy_secrets.md)	 - Secrets management commands



### runvoy status

Get the status of a command execution

### Options

```
  -h, --help   help for status
```

### Options inherited from parent commands

```
      --debug            Enable debugging logs
      --timeout string   Timeout for command execution (e.g., 10m, 30s, 1h) (default "10m")
      --verbose          Verbose output
```

### SEE ALSO

* [runvoy](runvoy.md)	 - runvoy



### runvoy users

User management commands

### Options

```
  -h, --help   help for users
```

### Options inherited from parent commands

```
      --debug            Enable debugging logs
      --timeout string   Timeout for command execution (e.g., 10m, 30s, 1h) (default "10m")
      --verbose          Verbose output
```

### SEE ALSO

* [runvoy](runvoy.md)	 - runvoy
* [runvoy users create](runvoy_users_create.md)	 - Create a new user
* [runvoy users list](runvoy_users_list.md)	 - List all users
* [runvoy users revoke](runvoy_users_revoke.md)	 - Revoke a user's API key



#### runvoy users create

Create a new user

Create a new user with the given email

**Examples:**

```bash
  - runvoy users create alice@example.com
  - runvoy users create bob@another-example.com
```

### Options

```
  -h, --help   help for create
```

### Options inherited from parent commands

```
      --debug            Enable debugging logs
      --timeout string   Timeout for command execution (e.g., 10m, 30s, 1h) (default "10m")
      --verbose          Verbose output
```

### SEE ALSO

* [runvoy users](runvoy_users.md)	 - User management commands



#### runvoy users list

List all users

List all users in the system with their basic information

**Examples:**

```bash
  - runvoy users list
```

### Options

```
  -h, --help   help for list
```

### Options inherited from parent commands

```
      --debug            Enable debugging logs
      --timeout string   Timeout for command execution (e.g., 10m, 30s, 1h) (default "10m")
      --verbose          Verbose output
```

### SEE ALSO

* [runvoy users](runvoy_users.md)	 - User management commands



#### runvoy users revoke

Revoke a user's API key

### Options

```
  -h, --help   help for revoke
```

### Options inherited from parent commands

```
      --debug            Enable debugging logs
      --timeout string   Timeout for command execution (e.g., 10m, 30s, 1h) (default "10m")
      --verbose          Verbose output
```

### SEE ALSO

* [runvoy users](runvoy_users.md)	 - User management commands



### runvoy version

Show the version of the CLI

### Options

```
  -h, --help   help for version
```

### Options inherited from parent commands

```
      --debug            Enable debugging logs
      --timeout string   Timeout for command execution (e.g., 10m, 30s, 1h) (default "10m")
      --verbose          Verbose output
```

### SEE ALSO

* [runvoy](runvoy.md)	 - runvoy



