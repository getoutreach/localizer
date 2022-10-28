// Copyright 2022 Outreach Corporation. All Rights Reserved.
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

// Description: This file has the package hostfile.
package hostsfile

import (
	"bytes"
	"context"
	"os"
	"reflect"
	"testing"

	"github.com/benbjohnson/clock"
	"github.com/google/go-cmp/cmp"
	"github.com/pkg/errors"
)

func TestNew(t *testing.T) {
	f, err := New("", "")
	if err != nil {
		t.Error(errors.Wrap(err, "failed to load with defaults"))
	}

	if f.contents == nil {
		t.Error("attached file was nil, should be populated")
	}
}

func TestFile_Load(t *testing.T) {
	f, err := New("./testdata/load/hosts-with-no-block.hosts", "")
	if err != nil {
		t.Error(err)
	}

	err = f.Load(context.Background())
	if err != nil {
		t.Error(errors.Wrap(err, "failed to load hosts file with no blocks"))
	}

	// load one with no metadata
	f, err = New("./testdata/load/hosts-with-block-no-metadata.hosts", "")
	if err != nil {
		t.Error(err)
	}

	err = f.Load(context.Background())
	if err == nil {
		t.Error("loaded hosts with no metadata block")
	}

	// load a valid hosts file
	f, err = New("./testdata/load/hosts-with-block.hosts", "")
	if err != nil {
		t.Error(err)
	}

	err = f.Load(context.Background())
	if err != nil {
		t.Error(errors.Wrap(err, "failed to load valid hosts file"))
	}

	expected := map[string]*HostLine{
		"127.0.0.1": {
			Addresses: []string{
				"hello-world",
			},
		},
	}

	if !reflect.DeepEqual(f.hostsFile, expected) {
		t.Error("expected: ", cmp.Diff(expected, f.hostsFile))
	}
}

func TestFile_Marshal(t *testing.T) {
	// load a valid hosts file
	filePath := "./testdata/load/hosts-with-block.hosts"
	f, err := New(filePath, "")
	if err != nil {
		t.Error(err)
	}
	f.clock = clock.NewMock()

	err = f.Load(context.Background())
	if err != nil {
		t.Error(errors.Wrap(err, "failed to load valid hosts file"))
	}

	b, err := f.Marshal(context.Background())
	if err != nil {
		t.Error(errors.Wrap(err, "failed to marshal hosts file"))
	}

	if !reflect.DeepEqual(f.contents, b) {
		t.Error("expected: ", cmp.Diff(f.contents, b))
	}

	// load a hosts file with no block, test that it gets added
	filePath = "./testdata/load/hosts-with-no-block.hosts"
	f, err = New(filePath, "")
	if err != nil {
		t.Error(err)
	}
	f.clock = clock.NewMock()

	err = f.Load(context.Background())
	if err != nil {
		t.Error(errors.Wrap(err, "failed to load valid hosts file"))
	}

	b, err = f.Marshal(context.Background())
	if err != nil {
		t.Error(errors.Wrap(err, "failed to marshal hosts file"))
	}

	expected := bytes.Join([][]byte{
		f.contents,
		[]byte("###start-hostfile\n###{\"blockName\":\"localizer\",\"last_modified_at\":\"1970-01-01T00:00:00Z\"}\n###end-hostfile"),
	}, []byte("\n"))

	if !reflect.DeepEqual(expected, b) {
		t.Error("expected: ", cmp.Diff(expected, b))
	}

	// modify a hosts file outside of our library, ensure the changes
	// are kept when Marshal'd
	filePath = "./testdata/load/hosts-with-block.hosts"
	f, err = New(filePath, "")
	if err != nil {
		t.Error(err)
	}
	f.clock = clock.NewMock()

	err = f.Load(context.Background())
	if err != nil {
		t.Error(errors.Wrap(err, "failed to load valid hosts file"))
	}

	// append a new entry
	f.contents = append([]byte(
		"127.0.0.1 helloworld.name.io\n",
	), f.contents...)

	b, err = f.Marshal(context.Background())
	if err != nil {
		t.Error(errors.Wrap(err, "failed to marshal hosts file"))
	}

	if !reflect.DeepEqual(b, f.contents) {
		t.Error("expected: ", cmp.Diff(b, f.contents))
	}
}

// Ensure that we don't corrupt hosts files that have data inside of
// a block already. This was triggering it in the past. This also ensures
// that our hosts file is ordered
func TestFile_HandleCorruption(t *testing.T) {
	f, err := New("./testdata/load/hosts-with-block-corrupt.hosts", "")
	if err != nil {
		t.Error(err)
	}
	f.clock = clock.NewMock()

	err = f.Load(context.Background())
	if err != nil {
		t.Error(errors.Wrap(err, "failed to load valid hosts file"))
	}

	b, err := f.Marshal(context.Background())
	if err != nil {
		t.Error(errors.Wrap(err, "failed to marshal hosts file"))
	}

	origContents, err := os.ReadFile("./testdata/load/hosts-with-block-corrupt.hosts")
	if err != nil {
		t.Fatal(err)
	}

	expected := origContents

	if !reflect.DeepEqual(expected, b) {
		t.Error("expected: ", cmp.Diff(expected, b))
	}
}

func TestFile_AddHosts(t *testing.T) {
	f, err := New("./testdata/load/hosts-with-block.hosts", "")
	if err != nil {
		t.Error(err)
	}

	err = f.Load(context.Background())
	if err != nil {
		t.Error(errors.Wrap(err, "failed to load valid hosts file"))
	}

	err = f.AddHosts("127.0.1.1", []string{"i-am-a-hostname"})
	if err != nil {
		t.Error(errors.Wrap(err, "failed to add a single address and hostname"))
	}

	err = f.AddHosts("127.0.1.2", []string{"i-am-a-hostname", "i-am-another-hostname"})
	if err != nil {
		t.Error(errors.Wrap(err, "failed to add a single address and hostname"))
	}

	err = f.AddHosts("127.0.1.2", []string{"i-am-a-hostname", "i-am-another hostname"})
	if err == nil {
		t.Error("allowed a invalid dns name")
	}

	b, err := f.Marshal(context.Background())
	if err != nil {
		t.Error(errors.Wrap(err, "failed to marshal hosts file"))
	}

	f = NewWithContents("", b)
	err = f.Load(context.Background())
	if err != nil {
		t.Error(errors.Wrap(err, "failed to load hosts"))
	}

	expected := map[string]*HostLine{
		"127.0.0.1": {
			Addresses: []string{
				"hello-world",
			},
		},
		"127.0.1.1": {
			Addresses: []string{
				"i-am-a-hostname",
			},
		},
		"127.0.1.2": {
			Addresses: []string{"i-am-a-hostname", "i-am-another-hostname"},
		},
	}
	if !reflect.DeepEqual(f.hostsFile, expected) {
		t.Error("expected: ", cmp.Diff(f.contents, b))
	}
}

func TestFile_RemoveHosts(t *testing.T) {
	f, err := New("./testdata/load/hosts-with-block.hosts", "")
	if err != nil {
		t.Error(err)
	}

	err = f.Load(context.Background())
	if err != nil {
		t.Error(errors.Wrap(err, "failed to load valid hosts file"))
	}

	err = f.RemoveAddress("127.0.0.1")
	if err != nil {
		t.Error(errors.Wrap(err, "failed to remove an address"))
	}

	b, err := f.Marshal(context.Background())
	if err != nil {
		t.Error(errors.Wrap(err, "failed to marshal"))
	}

	f = NewWithContents("", b)
	err = f.Load(context.Background())
	if err != nil {
		t.Error(errors.Wrap(err, "failed to load hosts"))
	}

	expected := map[string]*HostLine{}
	if !reflect.DeepEqual(f.hostsFile, expected) {
		t.Error("expected: ", cmp.Diff(f.contents, b))
	}
}
