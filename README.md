# Docker Volume Plugin: AWS EFS

Docker plugin for AWS EFS (Elastic File System)

This plugin will:

* Detect the VPC Subnet which the AWS host is placed (uses the DescribeInstance API endpoint)
* Automatically detect the region which the AWS host is placed
* Create an EFS Filesystem if it does not exist
* Create an EFS Mount Point if it does not exist
* MOunt to the local filesystem and into the container environment

## Roadmap

* Support for containers sharing the same EFS Filesystem (without checking out multiple times on the same host).
* CLI arg for custom EFS Security group (other than the default)

## Requirements

* NFS tools installed on the host (http://docs.aws.amazon.com/efs/latest/ug/mounting-fs.html)
* Docker 1.8+

## Usage

1. Start the plugin.

```bash
$ sudo ./docker-volumes-efs
```

2. Start a container with this plugin as the file storage backend

```bash
$ docker run --rm -it --volume-driver=efs -v foo:/no busybox
```
