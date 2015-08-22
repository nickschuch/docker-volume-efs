# Docker Volume Plugin: AWS EFS

Docker plugin for AWS EFS (Elastic File System)

This plugin will:

* Detect the VPC Subnet which the AWS host is placed (uses the DescribeInstance API endpoint)
* Automatically detect the region which the AWS host is placed
* Create an EFS Filesystem if it does not exist
* Create an EFS Mount Point if it does not exist
* Mount to the local filesystem and into the container environment

## Acknowledgements

* https://github.com/SvenDowideit/docker-volumes-nfs (Used as bootstrap)

## Roadmap

* Support for containers sharing the same EFS Filesystem (without checking out multiple times on the same host).
* CLI arg for custom EFS Security group (other than the default)

## Requirements

* NFS tools installed on the host (http://docs.aws.amazon.com/efs/latest/ug/mounting-fs.html)
* Docker 1.8+

## Usage

**Start the plugin**

```bash
$ sudo ./docker-volumes-efs
```

**Start a container with this plugin as the file storage backend**

```bash
$ docker run --rm -it --volume-driver=efs -v foo:/no busybox
```

## AWS Credentials

**I will provide a sample IAM role soon**

Before using the tool, ensure that you've configured credentials. The best
way to configure credentials on a development machine is to use the
`~/.aws/credentials` file, which might look like:

```ini
[default]
aws_access_key_id = AKID1234567890
aws_secret_access_key = MY-SECRET-KEY
```

You can learn more about the credentials file from this
[blog post](http://blogs.aws.amazon.com/security/post/Tx3D6U6WSFGOK2H/A-New-and-Standardized-Way-to-Manage-Credentials-in-the-AWS-SDKs).

Alternatively, you can set the following environment variables:

```
AWS_ACCESS_KEY_ID=AKID1234567890
AWS_SECRET_ACCESS_KEY=MY-SECRET-KEY
```

