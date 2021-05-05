// Copyright 2020 Jared Allard
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.
package hostsfile

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/asaskevich/govalidator"
	"github.com/benbjohnson/clock"
	"github.com/pkg/errors"
)

type File struct {
	clock clock.Clock

	// if this came from a file, this will be populated
	fileLocation string
	contents     []byte
	blockName    string

	lock     sync.Mutex
	saveLock sync.Mutex

	// Normally you can have more than one ip address
	// assigned multiple times in a hosts file, but given
	// we're managing our own block, we can safely group
	// these entries together.
	hostsFile map[string]*HostLine
}

type Metadata struct {
	BlockName    string    `json:"blockName"`
	LastModified time.Time `json:"last_modified_at"`
}

type HostLine struct {
	Addresses []string
}

func NewWithContents(sectionName string, contents []byte) *File {
	if sectionName == "" {
		sectionName = "localizer"
	}

	return &File{
		clock:     clock.New(),
		contents:  contents,
		blockName: sectionName,

		hostsFile: make(map[string]*HostLine),
	}
}

// New parses a hosts file and managed a block inside of the
// hosts file.
func New(fileLocation, sectionName string) (*File, error) {
	if fileLocation == "" {
		fileLocation = "/etc/hosts"
	}

	b, err := ioutil.ReadFile(fileLocation)
	if err != nil {
		return nil, err
	}

	f := NewWithContents(sectionName, b)
	f.fileLocation = fileLocation

	return f, nil
}

func (f *File) parseMetadata(line string) (*Metadata, error) {
	// strip the comment block
	metadataStr := strings.Replace(line, "###", "", 1)

	var metadata *Metadata
	err := json.Unmarshal([]byte(metadataStr), &metadata)
	if err != nil {
		return nil, errors.Wrap(err, "failed to parse metadata, is the file corrupted?")
	}

	return metadata, nil
}

// Load loads the hosts file into memory, and parses it.
func (f *File) Load(ctx context.Context) error { //nolint:funlen
	f.lock.Lock()
	defer f.lock.Unlock()

	scanner := bufio.NewScanner(bytes.NewReader(f.contents))

	foundBlock := false

	for scanner.Scan() {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}

		line := scanner.Text()
		chunks := strings.Split(line, " ")
		if len(chunks) == 0 {
			continue
		}

		// process the block start
		switch chunks[0] {
		case "###start-hostfile":
			foundBlock = true

			// fetch the metadata
			scanner.Scan()

			m, err := f.parseMetadata(scanner.Text())
			if err != nil {
				return err
			}

			// if the block doesn't match the one we're looking for, ignore it
			if m.BlockName != f.blockName {
				continue
			}
		case "###end-hostfile":
			foundBlock = false
		}

		if !foundBlock {
			continue
		}

		// skip lines that don't have at least an ip address and one host
		if len(chunks) < 2 {
			continue
		}

		// ensure we have a valid ip address
		ip := net.ParseIP(chunks[0])
		if ip == nil {
			continue
		}

		f.hostsFile[ip.String()] = &HostLine{
			Addresses: chunks[1:],
		}
	}
	if scanner.Err() != nil {
		return scanner.Err()
	}

	return nil
}

func (f *File) generateBlock() (string, error) {
	contents := []string{}

	m, err := json.Marshal(&Metadata{
		BlockName:    f.blockName,
		LastModified: f.clock.Now().UTC(),
	})
	if err != nil {
		return "", err
	}

	// ensure the output is stable, convert the keys
	// into a sorted slice
	ipAddresses := make([]string, len(f.hostsFile))

	i := 0
	for ip := range f.hostsFile {
		ipAddresses[i] = ip
		i++
	}

	sort.Slice(ipAddresses, func(i, j int) bool {
		return bytes.Compare(net.ParseIP(ipAddresses[i]), net.ParseIP(ipAddresses[j])) < 0
	})

	contents = append(contents, "###start-hostfile", fmt.Sprintf("###%s", m))
	for _, ip := range ipAddresses {
		contents = append(
			contents,
			fmt.Sprintf("%s %s", ip, strings.Join(f.hostsFile[ip].Addresses, " ")),
		)
	}
	contents = append(contents, "###end-hostfile")

	return strings.Join(contents, "\n"), nil
}

// Marshal renders a hosts file from memory.
func (f *File) Marshal(ctx context.Context) ([]byte, error) { //nolint:funlen
	f.lock.Lock()
	defer f.lock.Unlock()

	scanner := bufio.NewScanner(bytes.NewReader(f.contents))

	contents := []string{}
	wroteBlock := false

	copyLines := true
	for scanner.Scan() {
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		default:
		}

		line := scanner.Text()
		chunks := strings.Split(line, " ")

		// process the block start
		if len(chunks) != 0 {
			switch chunks[0] {
			case "###start-hostfile":
				scanner.Scan()
				m, err := f.parseMetadata(scanner.Text())
				if err != nil {
					return nil, err
				}

				if m.BlockName != f.blockName {
					continue
				}

				// write the blocks' contents
				wroteBlock = true
				b, err := f.generateBlock()
				if err != nil {
					return nil, errors.Wrap(err, "failed to generate hosts entries")
				}
				contents = append(contents, b)

				// discard lines until end block is found
				copyLines = false
			case "###end-hostfile":
				copyLines = true
				continue
			}
		}

		if !copyLines {
			continue
		}

		contents = append(contents, scanner.Text())
	}
	if scanner.Err() != nil {
		return nil, scanner.Err()
	}

	// if we never wrote the block, then append it to the end of the file
	if !wroteBlock {
		b, err := f.generateBlock()
		if err != nil {
			return nil, errors.Wrap(err, "failed to generate hosts entries")
		}

		contents = append(contents, b)
	}

	return []byte(strings.Join(contents, "\n")), nil
}

// Save marshalls the hosts file and then saves it to disk.
func (f *File) Save(ctx context.Context) error {
	if f.fileLocation == "" {
		return fmt.Errorf("can't write, was not loaded from a file")
	}

	var b []byte
	var err error
	if f.fileLocation != "" {
		f.lock.Lock()
		// re-read the hosts file to get potential
		// changes outside of our block
		//nolint:govet // Why: We're OK shadowing err
		b, err := ioutil.ReadFile(f.fileLocation)
		if err != nil {
			return err
		}
		f.contents = b
		f.lock.Unlock()
	}

	b, err = f.Marshal(ctx)
	if err != nil {
		return errors.Wrap(err, "failed to marshal hostsfile")
	}

	// ensure we don't write to the file at the same time
	f.saveLock.Lock()
	defer f.saveLock.Unlock()

	//nolint:gosec // We should eventually just use the existing perms here
	return ioutil.WriteFile(f.fileLocation, b, 0644)
}

// AddHosts adds a line into the hosts file for the given hosts to resolve
// to specified IP. Any existing hosts are replaced.
func (f *File) AddHosts(ipAddress string, hosts []string) error {
	f.lock.Lock()
	defer f.lock.Unlock()

	for _, h := range hosts {
		if !govalidator.IsDNSName(h) {
			return fmt.Errorf("'%s' is not a valid dns name", h)
		}
	}
	f.hostsFile[ipAddress] = &HostLine{Addresses: hosts}
	return nil
}

// RemoveAddress removes a given address and all hosts associated with it
// from the hosts file
func (f *File) RemoveAddress(ipAddress string) error {
	f.lock.Lock()
	defer f.lock.Unlock()

	delete(f.hostsFile, ipAddress)
	return nil
}
