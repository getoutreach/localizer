package hostsfile

import (
	"context"
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

	err = f.Load(context.Background())
	if err != nil {
		t.Error(errors.Wrap(err, "failed to load valid hosts file"))
	}

	f.clock = clock.NewMock()
	b, err := f.Marshal(context.Background())
	if err != nil {
		t.Error(errors.Wrap(err, "failed to marshal hosts file"))
	}

	// TODO: How do we test the generated contents when the date is generated?
	if !reflect.DeepEqual(f.contents, b) {
		t.Error("expected: ", cmp.Diff(f.contents, b))
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
