package main

import (
	"fmt"
	"log"
	"os"
	"strings"
	"time"

	"github.com/pkg/sftp"
	"golang.org/x/crypto/ssh"
)

type Backup struct {
	Name         string
	Files        []*File
	TotalSize    int64
	LastModified time.Time
}

type File struct {
	Path string
	Size int64
}

func (f *File) IsImmutable() bool {
	if strings.Contains(f.Path, "/objects/") {
		return true
	}

	if strings.HasSuffix(f.Path, ".pack") || strings.HasSuffix(f.Path, ".index") {
		return true
	}

	return false
}

func Connect(host string, user string, password string) (*sftp.Client, error) {
	sshConfig := &ssh.ClientConfig{
		User: user,
		Auth: []ssh.AuthMethod{
			ssh.Password(password),
		},
		HostKeyCallback: ssh.InsecureIgnoreHostKey(),
	}

	client, err := ssh.Dial("tcp", host, sshConfig)
	if err != nil {
		return nil, err
	}

	sftp, err := sftp.NewClient(client)
	if err != nil {
		return nil, err
	}

	return sftp, nil
}

func main() {
	user := os.Getenv("USER")
	password := os.Getenv("PASSWORD")
	host := os.Getenv("HOST")

	client, err := Connect(host, user, password)
	if err != nil {
		log.Fatal(err)
	}
	log.Println("Successfully connected to ssh server.")

	// All the backups we found
	backups := []*Backup{}

	// List the backups
	directories, err := client.ReadDir("/")
	if err != nil {
		log.Fatal(err)
	}

	for _, d := range directories {
		name := strings.TrimPrefix(d.Name(), "/")
		backup := &Backup{
			Name:      name,
			Files:     []*File{},
			TotalSize: 0,
		}

		// Walk all the files in the backup directory
		w := client.Walk(name)
		for w.Step() {
			if w.Err() != nil {
				continue
			}

			info := w.Stat()
			path := w.Path()
			modTime := info.ModTime()

			// Exclude directories
			if info.IsDir() {
				continue
			}

			file := &File{
				Path: path,
				Size: info.Size(),
			}

			backup.Files = append(backup.Files, file)

			// Update the total size
			backup.TotalSize = backup.TotalSize + info.Size()

			// Update the last modified time
			backup.LastModified = max(backup.LastModified, modTime)
		}

		backups = append(backups, backup)
	}

	for _, b := range backups {
		immutableSize := int64(0)
		mutableSize := int64(0)

		for _, f := range b.Files {
			if f.IsImmutable() {
				immutableSize = immutableSize + f.Size
			} else {
				mutableSize = mutableSize + f.Size
			}
		}

		fmt.Printf("%s (%d files), %d bytes (mutable), %d bytes (immutable), last modified %+v\n", b.Name, len(b.Files), mutableSize, immutableSize, b.LastModified)
	}
}

func max(a time.Time, b time.Time) time.Time {
	if a.After(b) {
		return a
	}

	return b
}
