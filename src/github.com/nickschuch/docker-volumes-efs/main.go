package main

import (
	"log"
	"os"
	"path/filepath"
	"strings"

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
	defaultDir    = filepath.Join(dkvolume.DefaultDockerRootDirectory, pluginId)

	// CLI Arguments.
	cliRoot     = kingpin.Flag("root", "EFS volumes root directory.").Default(defaultDir).String()
	cliSecurity = kingpin.Flag("security", "Security group to be assigned to new EFS Mount points.").Default("").OverrideDefaultFromEnvar("DOCKER_VOLUMES_EFS_SECURITY").String()
	cliVerbose  = kingpin.Flag("verbose", "Show verbose logging.").Bool()
)

type DriverEFS struct {
	Root   string
	Region string
	Subnet string
}

func (d DriverEFS) Create(r dkvolume.Request) dkvolume.Response {
	log.Printf("Create %v\n", r)
	return dkvolume.Response{}
}

func (d DriverEFS) Remove(r dkvolume.Request) dkvolume.Response {
	log.Printf("Remove %v\n", r)
	return dkvolume.Response{}
}

func (d DriverEFS) Path(r dkvolume.Request) dkvolume.Response {
	log.Printf("Path %v\n", r)
	return dkvolume.Response{Mountpoint: filepath.Join(d.Root, r.Name)}
}

func (d DriverEFS) Mount(r dkvolume.Request) dkvolume.Response {
	p := filepath.Join(d.Root, r.Name)

	// Check if we need to unmount this volume from the host.
	volumes, err := GetContainerByMount(p)
	if err != nil {
		return dkvolume.Response{Err: err.Error()}
	}

	if len(volumes) > 0 {
		log.Println("Volume exists, using that for this container instance")
		return dkvolume.Response{}
	}

	e := efs.New(&aws.Config{Region: aws.String(d.Region)})

	m, err := GetEFS(e, d.Subnet, r.Name)
	if err != nil {
		return dkvolume.Response{Err: err.Error()}
	}

	if err := os.MkdirAll(p, 0755); err != nil {
		return dkvolume.Response{Err: err.Error()}
	}

	// Mount the EFS volume to the local filesystem.
	// @todo, Swap this out with an NFS client library.
	if err := Exec("mount", "-t", "nfs4", m+":/", p); err != nil {
		return dkvolume.Response{Err: err.Error()}
	}

	return dkvolume.Response{Mountpoint: p}
}

func (d DriverEFS) Unmount(r dkvolume.Request) dkvolume.Response {
	p := filepath.Join(d.Root, r.Name)

	// Check if we need to unmount this volume from the host.
	volumes, err := GetContainerByMount(p)
	if err != nil {
		return dkvolume.Response{Err: err.Error()}
	}

	if len(volumes) > 0 {
		log.Println("Volume still in use, keeping it")
		return dkvolume.Response{}
	}

	log.Println("Unmount %s\n", p)
	err = Exec("umount", p)
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

	subnet, err := GetSubnet(e, i)
	if err != nil {
		panic(err)
	}

	d := DriverEFS{
		Root:   *cliRoot,
		Region: region,
		Subnet: subnet,
	}
	h := dkvolume.NewHandler(d)
	log.Printf("Listening on %s\n", socketAddress)
	log.Println(h.ServeUnix("root", socketAddress))
}
