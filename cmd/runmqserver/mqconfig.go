/*
© Copyright IBM Corporation 2017

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
package main

import (
	"io/ioutil"
	"log"
	"os/user"
	"runtime"
	"strings"

	"github.com/ibm-messaging/mq-container/internal/capabilities"
	"golang.org/x/sys/unix"
)

// fsTypes contains file system identifier codes.
// This code will not compile on some operating systems - Linux only.
var fsTypes = map[int64]string{
	0x61756673: "aufs",
	0xef53:     "ext",
	0x6969:     "nfs",
	0x65735546: "fuse",
	0x9123683e: "btrfs",
	0x01021994: "tmpfs",
	0x794c7630: "overlayfs",
}

func logBaseImage() error {
	buf, err := ioutil.ReadFile("/etc/os-release")
	if err != nil {
		return err
	}
	lines := strings.Split(string(buf), "\n")
	for _, l := range lines {
		if strings.HasPrefix(l, "PRETTY_NAME=") {
			words := strings.Split(l, "\"")
			if len(words) >= 2 {
				log.Printf("Base image detected: %v", words[1])
				return nil
			}
		}
	}
	return nil
}

func logUser() {
	u, err := user.Current()
	if err == nil {
		log.Printf("Running as user ID %v (%v) with primary group %v", u.Uid, u.Name, u.Gid)
	}
}

func logCapabilities() {
	status, err := readProc("/proc/1/status")
	if err != nil {
		// Ignore
		return
	}
	caps, err := capabilities.DetectCapabilities(status)
	if err == nil {
		log.Printf("Detected capabilities: %v", strings.Join(caps, ","))
	}
}

func readProc(filename string) (value string, err error) {
	buf, err := ioutil.ReadFile(filename)
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(string(buf)), nil
}

func readMounts() error {
	all, err := readProc("/proc/mounts")
	if err != nil {
		log.Println("Error: Couldn't read /proc/mounts")
		return err
	}
	lines := strings.Split(all, "\n")
	detected := false
	for i := range lines {
		parts := strings.Split(lines[i], " ")
		//dev := parts[0]
		mountPoint := parts[1]
		fsType := parts[2]
		if strings.Contains(mountPoint, "/mnt") {
			log.Printf("Detected '%v' volume mounted to %v", fsType, mountPoint)
			detected = true
		}
	}
	if !detected {
		log.Println("No volume detected. Persistent messages may be lost")
	} else {
		checkFS("/mnt/mqm")
	}
	return nil
}

func checkFS(path string) {
	statfs := &unix.Statfs_t{}
	err := unix.Statfs(path, statfs)
	if err != nil {
		log.Println(err)
		return
	}
	t := fsTypes[statfs.Type]
	switch t {
	case "aufs", "overlayfs", "tmpfs":
		log.Fatalf("Error: %v uses unsupported filesystem type %v", path, t)
	default:
		log.Printf("Detected %v has filesystem type '%v'", path, t)
	}
}

func logConfig() {
	log.Printf("CPU architecture: %v", runtime.GOARCH)
	if runtime.GOOS == "linux" {
		var err error
		osr, err := readProc("/proc/sys/kernel/osrelease")
		if err != nil {
			log.Println(err)
		} else {
			log.Printf("Linux kernel version: %v", osr)
		}
		logBaseImage()
		fileMax, err := readProc("/proc/sys/fs/file-max")
		if err != nil {
			log.Println(err)
		} else {
			log.Printf("Maximum file handles: %v", fileMax)
		}
		logUser()
		logCapabilities()
		readMounts()
	} else {
		log.Fatalf("Unsupported platform: %v", runtime.GOOS)
	}
}
