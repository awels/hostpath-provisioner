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
	"context"
	"os"
	"strconv"

	"github.com/alecthomas/kingpin/v2"
	"github.com/kopia/kopia/cli"
	"k8s.io/klog/v2"
)

type Kopia struct {
	nodeName   string
	path       string
	sourcePath string
}

func (k *Kopia) Initialize() error {
	if err := ensurePathExists(k.path); err != nil {
		return err
	}
	app := cli.NewApp()
	kp := kingpin.New("kopia", "Kopia - Fast And Secure Open-Source Backup").Author("http://kopia.github.io/")
	kp.Command("path", k.path)
	kp.Command("owner-uid", strconv.Itoa(os.Getuid()))
	kp.Command("owner-gid", strconv.Itoa(os.Getgid()))
	kp.Command("file-mode", "0600")
	kp.Command("dir-mode", "0700")
	kp.Command("flat", "false")
	kp.Command("list-parallelism", "1")
	out, stderr, wait, _ := app.RunSubcommand(context.TODO(), kp, os.Stdin, []string{"repository", "create", "filesystem", "--path", k.path})
	klog.V(1).Infof("Kopia stdout: %v", out)
	klog.V(1).Infof("Kopia stderr: %v", stderr)
	if err := wait(); err != nil {
		klog.Errorf("Failed to create kopia snapshot repo: %v", err)
		return err
	}
	klog.V(1).Info("Successfully created kopia snapshot repo")
	return nil
}
