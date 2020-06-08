EC2 CLI
=======

Run adhoc workloads using EC2 and destroy them upon completion

## Examples

```
ec2-runner run \
  --ami-filter "owner-alias=amazon" \
  --ami-filter "name=amzn2-ami-hvm*x86_64-ebs" \
  --tag "Name=Hello World" \
  --subnet-filter "tag:Environment=qa" \
  --subnet-filter "tag:Type=private" \
  --security-group-filter "group-name=qa_private" \
  --type t2.micro \
  echo "Hello world"
```

## Usage

```text

Usage:
  ec2-runner run [flags]

Flags:
      --ami string                          AMI name. Supports wildcards. Newest image is returned
      --ami-filter stringArray              'Key=Value' filters for your AMI
      --ami-id string                       AMI ID, overriding ami-filter or ami
  -c, --count int                           Number of instances to invoke (default 1)
      --dry-run                             Show details about the instance it would start, but don't actually start it
      --entrypoint string                   path to entrypoint script
  -h, --help                                help for run
  -i, --identify-file string                If using ssh-key, pass in the identitiy file
      --instance-profile string             Role to attach to your instance
      --no-terminate                        Do not terminate the instance upon completion (default true)
      --no-wait-cloud-init                  Do not wait for user-data to complete before invoking entrypoint and command (default true)
      --security-group stringArray          Security group name
      --security-group-filter stringArray   Filters for your Security Groups. Syntax: Name=string,Values=string,string ...
      --ssh-key string                      (optional) use this AWS SSH key. If omitted, an ephemeral key will be created
      --ssh-port int                        SSH port (default 22)
      --subnet string                       Subnet name. First match is returned
      --subnet-filter stringArray           'Key=Value' filters for your subnet
      --subnet-id string                    Subnet ID, overriding subnet-filter or subnet
      --tag stringArray                     Key=Value pair
  -t, --type string                         instance type (default "t2.micro")
      --user string                         SSH user to connect to your instance with (default "ec2-user")
      --user-data string                    path to user-data script

```
