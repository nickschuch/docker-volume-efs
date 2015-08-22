package main

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	log "github.com/Sirupsen/logrus"
	"github.com/alecthomas/kingpin"
	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/ec2metadata"
	"github.com/aws/aws-sdk-go/service/ec2"
	"github.com/aws/aws-sdk-go/service/efs"
	"github.com/calavera/docker-volume-api"
)

const (
	pluginId = "efs"
)

var (
	socketAddress = filepath.Join("/run/docker/plugins/", strings.Join([]string{pluginId, ".sock"}, ""))
	defaultDir    = filepath.Join(dkvolume.DefaultDockerRootDirectory, strings.Join([]string{"_", pluginId}, ""))

	// CLI Arguments.
	cliRoot    = kingpin.Flag("root", "EFS volumes root directory.").Default(defaultDir).String()
	cliSubnet  = kingpin.Flag("subnet", "VPC subnet.").Required().String()
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

	fsParams := &efs.CreateFileSystemInput{
		CreationToken: aws.String(r.Name),
	}
	fsResp, err := e.CreateFileSystem(fsParams)
	if err != nil {
		return dkvolume.Response{Err: err.Error()}
	}

	mntParams := &efs.CreateMountTargetInput{
		FileSystemId: fsResp.FileSystemId,
		SubnetId:     aws.String(*cliSubnet),
	}
	mntResp, err := e.CreateMountTarget(mntParams)
	if err != nil {
		return dkvolume.Response{Err: err.Error()}
	}

	// Mount the EFS volume to the local filesystem.
	// @todo, Swap this out with an NFS client library.
	if err := run("mount", "-o", "port=2049,nolock,proto=tcp", *mntResp.IpAddress, p); err != nil {
		return dkvolume.Response{Err: err.Error()}
	}

	return dkvolume.Response{Mountpoint: p}
}

func (d efsDriver) Unmount(r dkvolume.Request) dkvolume.Response {
	p := filepath.Join(d.Root, r.Name)
	log.Printf("Unmount %s\n", p)

	if err := run("umount", p); err != nil {
		return dkvolume.Response{Err: err.Error()}
	}

	err := os.RemoveAll(p)
	return dkvolume.Response{Err: err.Error()}
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

	describeParams := &ec2.DescribeInstancesInput{
		InstanceIds: []*string{
			aws.String(i),
		},
		MaxResults: aws.Int64(1),
	}
	describeResp, err := e.DescribeInstances(describeParams)
	if err != nil {
		panic(err)
	}

	// Ensure we got a result from this query.
	if len(describeResp.Reservations) <= 0 {
		panic("Cannot find this host by AWS EC2 DescribeInstances API")
	}

	d := efsDriver{
		Root:   *cliRoot,
		Region: region,
		Subnet: *describeResp.Reservations[0].Instances[0].SubnetId,
	}
	h := dkvolume.NewHandler(d)
	log.Printf("Listening on %s\n", socketAddress)
	log.Println(h.ServeUnix("root", socketAddress))
}

func run(exe string, args ...string) error {
	cmd := exec.Command(exe, args...)
	if *cliVerbose {
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
		log.Printf("Executing: %v %v", exe, strings.Join(args, " "))
	}
	if err := cmd.Run(); err != nil {
		return err
	}
	return nil
}
