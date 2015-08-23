package main

import (
	"log"
	"time"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/service/efs"
)

const (
	efsAvail = "available"
)

// Helper function to get the EFS endpoint for mounting.
func GetEFS(e *efs.EFS, s string, n string) (string, error) {
	// Check if the EFS Filesystem already exists.
	fs, err := DescribeFilesystem(e, n)
	if err != nil {
		return "", err
	}

	if len(fs.FileSystems) > 0 {
		mnt, err := DescribeMountTarget(e, *fs.FileSystems[0].FileSystemId)
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
		newMnt, err := CreateMountTarget(e, *fs.FileSystems[0].FileSystemId, s)
		if err != nil {
			return "", err
		}

		log.Printf("Using existing EFS Mount point: %s", *newMnt.IpAddress)
		return *newMnt.IpAddress, nil
	}

	// We now have the go ahead to create one instead.
	newFs, err := CreateFilesystem(e, n)
	if err != nil {
		return "", err
	}
	newMnt, err := CreateMountTarget(e, *newFs.FileSystemId, s)
	if err != nil {
		return "", err
	}

	log.Printf("Created new EFS Filesytem with mount point: %s", *newMnt.IpAddress)
	return *newMnt.IpAddress, nil
}

// Helper function to create an EFS Filesystem.
func CreateFilesystem(e *efs.EFS, n string) (*efs.FileSystemDescription, error) {
	createParams := &efs.CreateFileSystemInput{
		CreationToken: aws.String(n),
	}
	createResp, err := e.CreateFileSystem(createParams)
	if err != nil {
		return nil, err
	}

	// Wait for the filesystem to become available.
	for {
		fs, err := DescribeFilesystem(e, n)
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
func DescribeFilesystem(e *efs.EFS, n string) (*efs.DescribeFileSystemsOutput, error) {
	params := &efs.DescribeFileSystemsInput{
		CreationToken: aws.String(n),
	}
	return e.DescribeFileSystems(params)
}

// Helper function to create an EFS Mount target.
func CreateMountTarget(e *efs.EFS, i string, s string) (*efs.MountTargetDescription, error) {
	var security []*string

	// Determine if we need to assign a security group to this mount point, otherwise defer
	// to the default group.
	if *cliSecurity != "" {
		security = []*string{
			cliSecurity,
		}
	}

	params := &efs.CreateMountTargetInput{
		FileSystemId:   aws.String(i),
		SubnetId:       aws.String(s),
		SecurityGroups: security,
	}
	resp, err := e.CreateMountTarget(params)
	if err != nil {
		return nil, err
	}

	// Wait for the mount point to become available.
	for {
		mnt, err := DescribeMountTarget(e, i)
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
func DescribeMountTarget(e *efs.EFS, i string) (*efs.DescribeMountTargetsOutput, error) {
	params := &efs.DescribeMountTargetsInput{
		FileSystemId: aws.String(i),
	}
	return e.DescribeMountTargets(params)
}
