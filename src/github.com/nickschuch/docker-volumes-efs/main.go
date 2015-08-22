package main

import (
	"errors"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/alecthomas/kingpin"
	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/ec2metadata"
	"github.com/aws/aws-sdk-go/service/ec2"
	"github.com/aws/aws-sdk-go/service/efs"
	"github.com/calavera/docker-volume-api"
)

const (
	pluginId = "efs"
	efsAvail = "available"
)

var (
	socketAddress = filepath.Join("/run/docker/plugins/", strings.Join([]string{pluginId, ".sock"}, ""))
	defaultDir    = filepath.Join(dkvolume.DefaultDockerRootDirectory, pluginId)

	// CLI Arguments.
	cliRoot    = kingpin.Flag("root", "EFS volumes root directory.").Default(defaultDir).String()
	cliVerbose = kingpin.Flag("verbose", "Show verbose logging.").Bool()
)

type efsDriver struct {
	Root   string
	Region string
	Subnet string
}

func (d efsDriver) Create(r dkvolume.Request) dkvolume.Response {
	log.Printf("Create %v\n", r)
	return dkvolume.Response{}
}

func (d efsDriver) Remove(r dkvolume.Request) dkvolume.Response {
	log.Printf("Remove %v\n", r)
	return dkvolume.Response{}
}

func (d efsDriver) Path(r dkvolume.Request) dkvolume.Response {
	log.Printf("Path %v\n", r)
	return dkvolume.Response{Mountpoint: filepath.Join(d.Root, r.Name)}
}

func (d efsDriver) Mount(r dkvolume.Request) dkvolume.Response {
	p := filepath.Join(d.Root, r.Name)
	e := efs.New(&aws.Config{Region: aws.String(d.Region)})

	m, err := getEFS(e, d.Subnet, r.Name)
	if err != nil {
		return dkvolume.Response{Err: err.Error()}
	}

	if err := os.MkdirAll(p, 0755); err != nil {
		return dkvolume.Response{Err: err.Error()}
	}

	// Mount the EFS volume to the local filesystem.
	// @todo, Swap this out with an NFS client library.
	if err := run("mount", "-t", "nfs4", m+":/", p); err != nil {
		return dkvolume.Response{Err: err.Error()}
	}

	return dkvolume.Response{Mountpoint: p}
}

func (d efsDriver) Unmount(r dkvolume.Request) dkvolume.Response {
	p := filepath.Join(d.Root, r.Name)
	log.Println("Unmount %s\n", p)

	err := run("umount", p)
	if err != nil {
		return dkvolume.Response{Err: err.Error()}
	}

	err = os.RemoveAll(p)
	if err != nil {
		return dkvolume.Response{Err: err.Error()}
	}

	return dkvolume.Response{}
}

func main() {
	kingpin.Parse()

	// Discovery the region which this instance resides. This will ensure the
	// EFS Filesystem gets created in the same region as this instance.
	metadata := ec2metadata.New(&ec2metadata.Config{})
	region, err := metadata.Region()
	if err != nil {
		panic(err)
	}

	// We need to determine which region this host lives in. That will allow us to spin
	// up EFS Filesystem within this region.
	e := ec2.New(&aws.Config{Region: aws.String(region)})

	i, err := metadata.GetMetadata("instance-id")
	if err != nil {
		panic(err)
	}

	subnet, err := getSubnet(e, i)
	if err != nil {
		panic(err)
	}

	d := efsDriver{
		Root:   *cliRoot,
		Region: region,
		Subnet: subnet,
	}
	h := dkvolume.NewHandler(d)
	log.Printf("Listening on %s\n", socketAddress)
	log.Println(h.ServeUnix("root", socketAddress))
}

// Helper function to get a subnet which an EC2 instance belong to.
func getSubnet(e *ec2.EC2, i string) (string, error) {
	describeParams := &ec2.DescribeInstancesInput{
		InstanceIds: []*string{
			aws.String(i),
		},
	}
	describeResp, err := e.DescribeInstances(describeParams)
	if err != nil {
		return "", err
	}

	// Ensure we got a result from this query.
	if len(describeResp.Reservations) <= 0 {
		return "", errors.New("Cannot find this host by AWS EC2 DescribeInstances API")
	}
	if len(describeResp.Reservations[0].Instances) <= 0 {
		return "", errors.New("Cannot find this host by AWS EC2 DescribeInstances API")
	}

	return *describeResp.Reservations[0].Instances[0].SubnetId, nil
}

// Helper function to get the EFS endpoint for mounting.
func getEFS(e *efs.EFS, s string, n string) (string, error) {
	// Check if the EFS Filesystem already exists.
	fs, err := describeFilesystem(e, n)
	if err != nil {
		return "", err
	}

	if len(fs.FileSystems) > 0 {
		mnt, err := describeMountTarget(e, *fs.FileSystems[0].FileSystemId)
		if err != nil {
			return "", err
		}

		// This means we do have a mount target and we don't need to worry about
		// creating one.
		if len(mnt.MountTargets) > 0 {
			return *mnt.MountTargets[0].IpAddress, nil
		}

		// In the off chance that we find outselves in a position where we don't have
		// a mount target for this EFS Filesystem we create one.
		newMnt, err := createMountTarget(e, *fs.FileSystems[0].FileSystemId, s)
		if err != nil {
			return "", err
		}

		log.Println("Using existing EFS Mount point: %s", *newMnt.IpAddress)
		return *newMnt.IpAddress, nil
	}

	// We now have the go ahead to create one instead.
	newFs, err := createFilesystem(e, n)
	if err != nil {
		return "", err
	}
	newMnt, err := createMountTarget(e, *newFs.FileSystemId, s)
	if err != nil {
		return "", err
	}

	log.Println("Created new EFS Filesytem with mount point: %s", *newMnt.IpAddress)
	return *newMnt.IpAddress, nil
}

// Helper function to create an EFS Filesystem.
func createFilesystem(e *efs.EFS, n string) (*efs.FileSystemDescription, error) {
	createParams := &efs.CreateFileSystemInput{
		CreationToken: aws.String(n),
	}
	createResp, err := e.CreateFileSystem(createParams)
	if err != nil {
		return nil, err
	}

	// Wait for the filesystem to become available.
	for {
		fs, err := describeFilesystem(e, n)
		if err != nil {
			return nil, err
		}
		if len(fs.FileSystems) > 0 {
			if *fs.FileSystems[0].LifeCycleState == efsAvail {
				break
			}
		}
		time.Sleep(10 * time.Second)
	}

	return createResp, nil
}

// Helper function to describe EFS Filesystems.
func describeFilesystem(e *efs.EFS, n string) (*efs.DescribeFileSystemsOutput, error) {
	params := &efs.DescribeFileSystemsInput{
		CreationToken: aws.String(n),
	}
	return e.DescribeFileSystems(params)
}

// Helper function to create an EFS Mount target.
func createMountTarget(e *efs.EFS, i string, s string) (*efs.MountTargetDescription, error) {
	params := &efs.CreateMountTargetInput{
		FileSystemId: aws.String(i),
		SubnetId:     aws.String(s),
	}
	resp, err := e.CreateMountTarget(params)
	if err != nil {
		return nil, err
	}

	// Wait for the mount point to become available.
	for {
		mnt, err := describeMountTarget(e, i)
		if err != nil {
			return nil, err
		}
		if len(mnt.MountTargets) > 0 {
			if *mnt.MountTargets[0].LifeCycleState == efsAvail {
				break
			}
		}
		time.Sleep(10 * time.Second)
	}

	return resp, nil
}

// Helper function to describe an EFS Mount target.
func describeMountTarget(e *efs.EFS, i string) (*efs.DescribeMountTargetsOutput, error) {
	params := &efs.DescribeMountTargetsInput{
		FileSystemId: aws.String(i),
	}
	return e.DescribeMountTargets(params)
}

// Helper function to execute a command.
func run(exe string, args ...string) error {
	cmd := exec.Command(exe, args...)
	if *cliVerbose {
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
	}
	if err := cmd.Run(); err != nil {
		return err
	}
	return nil
}
