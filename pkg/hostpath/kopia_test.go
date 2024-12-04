/*
Copyright 2024 The hostpath provisioner Authors.

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
	"os"
	"path/filepath"
	"testing"

	. "github.com/onsi/gomega"
	"go.uber.org/zap/zaptest"
)

func TestKopia_Initialize(t *testing.T) {
	RegisterTestingT(t)
	kopia := &Kopia{
		path:       "/invalid/path",
		sourcePath: "/invalid/path",
		nodeName:   "testnode",
	}
	err := kopia.Initialize()
	Expect(err).To(HaveOccurred())
}

func TestKopia_GetSnapshotById(t *testing.T) {
	RegisterTestingT(t)
	zaptest.NewLogger(t)

	tempDir, err := os.MkdirTemp(os.TempDir(), "")
	Expect(err).ToNot(HaveOccurred())
	defer os.RemoveAll(tempDir)
	kopia := &Kopia{
		path:       filepath.Join(tempDir, testSnapshotDir),
		sourcePath: filepath.Join(tempDir, testVolumeDir),
		nodeName:   "testnode",
	}
	err = kopia.Initialize()
	Expect(err).ToNot(HaveOccurred())
	// t.Run("snapshot does not exist", func(t *testing.T) {
	// 	snapshotId := testSnapshot
	// 	snapshot, err := kopia.GetSnapshotById(snapshotId)
	// 	Expect(err).ToNot(HaveOccurred())
	// 	Expect(snapshot).To(BeNil())
	// })
	// err = ensurePathExists(filepath.Join(kopia.sourcePath, testVolume))
	// Expect(err).ToNot(HaveOccurred())
	// snapshot, err := kopia.CreateSnapshot(testSnapshot, testVolume)
	// Expect(err).ToNot(HaveOccurred())
	// Expect(snapshot).ToNot(BeNil())
	// t.Run("snapshot exists", func(t *testing.T) {
	// 	snapshotId := testSnapshot
	// 	snapshot, err := kopia.GetSnapshotById(snapshotId)
	// 	Expect(err).ToNot(HaveOccurred())
	// 	Expect(snapshot).ToNot(BeNil())
	// 	Expect(snapshot.SnapshotId).To(Equal(snapshotId))
	// })
}
