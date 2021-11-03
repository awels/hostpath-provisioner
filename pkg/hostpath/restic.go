/*
Copyright 2021 The hostpath provisioner Authors.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package hostpath

import (
	"bufio"
	"bytes"
	"encoding/json"
	"fmt"
	"path/filepath"
	"time"

	"github.com/container-storage-interface/spec/lib/go/csi"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/timestamppb"
	"k8s.io/klog/v2"
	"k8s.io/utils/exec"
)

type SnapshotMeta struct {
	Time  string `json:"time"`
	Parent  string `json:"parent"`
	Tree  string `json:"tree"`
	Paths []string `json:"paths"`
	Hostname string `json:"hostname"`
	Username string `json:"username"`
	Uid uint `json:"uid"`
	Gid uint `json:"gid"`
	Tags []string `json:"tags"`
	Id string `json:"id"`
	ShortId string `json:"short_id"`
}

type snapshotFile struct {
    Name string `json:"name"`
    Type string `json:"type"`
    Path string `json:"path"`
    Uid uint `json:"uid"`
    Gid uint `json:"gid"`
    Size int64 `json:"size"`
    Mode uint `json:"mode"`
    Modified string `json:"mtime"`
    LastRead string `json:"atime"`
    Created string `json:"ctime"`
    StructTpe string `json:"struct_type"`
}

type SnapshotProvider interface {
	// Initialize initialize the provider.
	Initialize() error
	// GetSnapshotById gets the snapshot meta data of the specified snapshot id.
	GetSnapshotById(snapshotId string) (*csi.Snapshot, error)
	// GetSnapshotsByVolumeSourceId gets the snapshot meta data of the snapshots associated with the volume source id. All snapshots of a volume
	GetSnapshotsByVolumeSourceId(volumeSourceId string) ([]csi.Snapshot, error)
	// GetAllSnapshots gets all the snapshot meta data
	GetAllSnapshots() ([]csi.Snapshot, error)
	// CreateSnapshot creates a snapshot.
	CreateSnapshot(snapshotId, sourceVolumeId string) (*csi.Snapshot, error)
	// DeleteSnapshot removes a snapshot
	DeleteSnapshot(snapshotId string) error
	// RestoreSnapshot restores the content of the snapshot into the target path
	RestoreSnapshot(snapshotId, targetPath string) error 
}

type Restic struct {
	reponame string
	pwdFile string
	sourcebase string
	nodeName string
}

func (r *Restic) Initialize() error {
	cmd := []string{"restic", "init", "--password-file", r.pwdFile, "--repo", r.reponame}
	executor := exec.New()
	out, err := executor.Command(cmd[0], cmd[1:]...).CombinedOutput()
	if err != nil {
		return fmt.Errorf("error %s, %v",string(out), err)
	}
	klog.V(1).Info("Successfully created snapshot repo")
	return nil
}

func (r *Restic) GetSnapshotById(snapshotId string) (*csi.Snapshot, error) {
	if snapshots, err := r.getSnapshotsWithArgs("--tag", snapshotId); err != nil {
		return nil, err
	} else if len(snapshots) > 0 {
		return &snapshots[0], nil
	}
	return nil, nil
}

func (r *Restic) getSnapshotMetaById(snapshotId string) (*SnapshotMeta, error) {
	if snapshots, err := r.getSnapshotMetasWithArgs("--tag", snapshotId); err != nil {
		return nil, err
	} else if len(snapshots) > 0 {
		return &snapshots[0], nil
	}
	return nil, nil
}

func (r *Restic) GetSnapshotsByVolumeSourceId(volumeSourceId string) ([]csi.Snapshot, error) {
	return r.getSnapshotsWithArgs("--path", filepath.Join(r.sourcebase, volumeSourceId))
}

func (r *Restic) GetAllSnapshots() ([]csi.Snapshot, error) {
	return r.getSnapshotsWithArgs()
}

func (r *Restic) getSnapshotsWithArgs(extraArgs...string) ([]csi.Snapshot, error) {
	snapshots := make([]csi.Snapshot, 0)
	snapshotMetas, err := r.getSnapshotMetasWithArgs(extraArgs...)
	if err != nil {
		return snapshots, err
	}
	for _, meta := range snapshotMetas {
		snapshot, err := r.createSnapshotFromMeta(&meta)
		if err != nil {
			return snapshots, nil
		}
		snapshots = append(snapshots, *snapshot)
	}
	return snapshots, nil
}

func (r *Restic) getSnapshotMetasWithArgs(extraArgs...string) ([]SnapshotMeta, error) {
	cmd := []string{"restic", "snapshots", "--password-file", r.pwdFile, "--repo", r.reponame, "--json"}
	cmd = append(cmd, extraArgs...)
	executor := exec.New()
	out, err := executor.Command(cmd[0], cmd[1:]...).CombinedOutput()
	if err != nil {
		return nil, fmt.Errorf("%s, %v", out, err)
	}
	snapshotMetas := []SnapshotMeta{}
	if err := json.Unmarshal(out, &snapshotMetas); err != nil {
		return nil, fmt.Errorf("error getting snapshot by ID, %v", err)
	}
	return snapshotMetas, nil
}

func (r *Restic) getSnapshotSizeFromMeta(meta *SnapshotMeta) (int64, error) {
	cmd := []string{"restic", "ls", "-l", "--password-file", r.pwdFile, "--repo", r.reponame, "--json", meta.Id}
	executor := exec.New()
	out, err := executor.Command(cmd[0], cmd[1:]...).CombinedOutput()
	if err != nil {
		return int64(0), err
	}

	buf := bytes.NewBuffer(out)
	scanner := bufio.NewScanner(buf)
	// Skip first line as it is the snapshot meta data
	scanner.Scan()
	totalSize := int64(0)
	for scanner.Scan() {
		file := snapshotFile{}
		bytes := scanner.Bytes()
		if err := json.Unmarshal(bytes, &file); err != nil {
			return int64(0), err
		}
		totalSize += int64(file.Size)
	}
	return totalSize, nil
}

func (r *Restic) CreateSnapshot(snapshotId, sourceVolumeId string) (*csi.Snapshot, error) {
	// Check if the source volume exists, so we can snapshot it.
	sourceDir := filepath.Join(r.sourcebase, sourceVolumeId)
	if exists, err := checkPathExist(sourceDir); err != nil {
		return nil, err
	} else if exists {
		cmd := []string{"restic", "--password-file", r.pwdFile, "--repo", r.reponame, "--tag", snapshotId, "backup", sourceDir}
		executor := exec.New()
		out, err := executor.Command(cmd[0], cmd[1:]...).CombinedOutput()
		if err != nil {
			return nil, fmt.Errorf("%s, %v", out, err)
		}
		meta, err := r.getSnapshotMetaById(snapshotId)
		if err != nil {
			return nil, err
		}
		if handle, err := r.tagSnapshotWithNode(meta); err != nil {
			return nil, err
		} else {
			return r.GetSnapshotById(handle)
		}
	}
	return nil, fmt.Errorf("source volume %s not found, unable to create snapshot", sourceDir)
}

func (r *Restic) tagSnapshotWithNode(meta *SnapshotMeta) (string, error) {
	handle := fmt.Sprintf("%s-%s", r.nodeName, meta.ShortId)
	cmd := []string{"restic", "--password-file", r.pwdFile, "--repo", r.reponame, "tag", "--add", handle}
	executor := exec.New()
	out, err := executor.Command(cmd[0], cmd[1:]...).CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("%s, %v", out, err)
	}
	return handle, nil
}

func (r *Restic) DeleteSnapshot(snapshotId string) error {
	meta, err := r.getSnapshotMetaById(snapshotId)
	if err != nil {
		return err
	}
	if meta != nil {
		cmd := []string{"restic", "--password-file", r.pwdFile, "--repo", r.reponame, "forget", meta.Id}
		executor := exec.New()
		out, err := executor.Command(cmd[0], cmd[1:]...).CombinedOutput()
		if err != nil {
			return fmt.Errorf("%s", out)
		}
	}
	return nil
}

func (r *Restic) RestoreSnapshot(snapshotId, targetPath string) error {
	if meta, err := r.getSnapshotMetaById(snapshotId); err != nil {
		return err
	} else if meta != nil {
		cmd := []string{"restic", "--password-file", r.pwdFile, "--repo", r.reponame, "--target", targetPath, "restore", meta.Id}
		executor := exec.New()
		out, err := executor.Command(cmd[0], cmd[1:]...).CombinedOutput()
		if err != nil {
			return fmt.Errorf("%s", out)
		}
		return nil
	} else {
		return status.Error(codes.NotFound, "snapshot not found, unable to restore from it")
	}
}

func (r *Restic) createSnapshotFromMeta(meta *SnapshotMeta) (*csi.Snapshot, error) {
	sourceVolumeId := filepath.Base(meta.Paths[0])
	creationTime, err := time.Parse(time.RFC3339Nano, meta.Time)
	if err != nil {
		klog.V(1).Infof("Error getting snapshot creation time %v", err)
		return nil, err
	}
	size, err := r.getSnapshotSizeFromMeta(meta)
	if err != nil {
		klog.V(1).Infof("Error getting volume %s used size %v", sourceVolumeId, err)
		return nil, err
	}
	return &csi.Snapshot{
		SnapshotId: meta.Tags[0],
		SourceVolumeId: sourceVolumeId,
		CreationTime: timestamppb.New(creationTime),
		SizeBytes: size,
		ReadyToUse: true,
	}, nil
}

