package main

// This Source Code Form is subject to the terms of the Mozilla Public License, v. 2.0. If a copy of the MPL was not distributed with this file, You can obtain one at https://mozilla.org/MPL/2.0/.

import (
	"crypto/md5"
	"fmt"
	"net/http"
	"net/http/httptest"
	"path"
	"strings"
	"testing"
)

var starting_hash string = "2ccc62d64502f9e7f1231c5b228136d3ee0fa72c"
var md5_of_starting_file string = "0566ec561947146909cf40192cda39ec"
var md5_of_gitserve_at_first_tag string = "bc01be1e5c1fbdbe31ac89ae8fb154cd"
var md5_of_nested_testfile string = "d8e8fca2dc0f896fd7cb4cb0031ba249"

func TestDisplayingObject(t *testing.T) {
	first_commit, err := get_object(starting_hash, "gitserve.go")

	first_file_calculated_md5 := fmt.Sprintf("%x", md5.Sum(first_commit))

	if err != nil {
		t.Error(err)
	}
	if first_file_calculated_md5 != md5_of_starting_file {
		t.Errorf("%s came back- not %s\n", first_file_calculated_md5, md5_of_starting_file)
	}
}

func TestGetHumanNames(t *testing.T) {
	// `git show-ref` is going to be the source of this data
	// take off the leading 'refs/', and use the rest as the list of object sources
	// means the data source can be fed via `git fetch` (shows up under 'remotes')
	// or `git push` (data shows up under 'heads'). As long as the user is consistent,
	// and there's an interface to list refs, everything should be pretty reasonable,
	// though it's less concise, than, say, github's url structure (they can cheat &
	// require `git push`)
	var refs []string
	refs, err := get_refs()
	if err != nil {
		t.Error(err)
	}

	// Check that existing tags exist
	// Check that remotes/origin/master exists
	// This *is* a little dependent on where you get this from,
	// so maybe I should build a little dedicated demo submodule ...
	var rooted_tag, zeroth_version_tag, remote_master_branch bool = false, false, false
	for _, ref := range refs {
		if ref == "remotes/origin/master" {
			remote_master_branch = true
		} else if ref == "tags/0.0.0.0.1" {
			zeroth_version_tag = true
		} else if ref == "tags/rooted/tags/are/tricky" {
			rooted_tag = true
		}
	}
	if !rooted_tag {
		t.Error("didn't find tags/rooted/tags/are/tricky")
	} else if !zeroth_version_tag {
		t.Error("didn't find tags/0.0.0.0.1")
	} else if !remote_master_branch {
		t.Error("didn't find remotes/origin/master- this can be checkout dependent, sorry for the flaky test")
	}
}

func TestDisplayingMissingObject(t *testing.T) {
	first_commit, err := get_object(starting_hash, "quack")

	if err == nil {
		t.Error("This should be an error- this is not a legit file")
	}
	if first_commit != nil {
		t.Errorf("What are you doing returning content here? '%q'", first_commit)
	}
}

func TestDisplayingBadRoot(t *testing.T) {
	first_commit, err := get_object("invalid_hash", "gitserve.go")

	if err == nil {
		t.Error("This should be an error- this is not a legit hash")
	}
	if first_commit != nil {
		t.Errorf("What are you doing returning content here? '%q'", first_commit)
	}
}

func TestPathRsplit(t *testing.T) {
	for _, test_case := range []struct {
		Path, OutputA, OutputB string
	}{
		{"foo", "foo", ""},
		{"/foo", "foo", ""},
		{"foo/", "foo", ""},
		{"", "", ""},
		{"/", "", ""},
		{"/foo/bar/baz", "foo", "bar/baz"},
		{"foo/bar/baz", "foo", "bar/baz"},
	} {
		root, branch := PathRsplit(test_case.Path)
		if root != test_case.OutputA {
			t.Error("root", root, "does not match", test_case.OutputA, "from", test_case.Path)
		}
		if branch != test_case.OutputB {
			t.Error("branch", branch, "does not match", test_case.OutputB, "from", test_case.Path)
		}
	}
}

func TestHttpTreeApi(t *testing.T) {
	// If you go to http://server:port/blob/master, you might hope to get a file
	// listing instead of a 404
	for _, test_case := range []struct {
		Blob, Path      string
		ExpectedEntries []string
	}{
		{"tags/rooted/tags/may/confuse", "/", []string{"gitserve.go", "gitserve_test.go"}},
		{"2ccc6", "/", []string{"gitserve.go"}},
	} {
		req, err := http.NewRequest("GET", path.Join("/blob/", test_case.Blob, test_case.Path), nil)
		if err != nil {
			t.Error("Test request failed", err)
		}
		w := httptest.NewRecorder()
		servePath(w, req)

		listing := w.Body.String()
		t.Log(path.Join("/blob/", test_case.Blob, test_case.Path))
		t.Log("Listing: ", listing)
		for _, entry := range test_case.ExpectedEntries {
			if !strings.Contains(listing, entry) {
				t.Error("Output not what we expected- missing ", entry, " from ", test_case.Path, " @ ", test_case.Blob)
			}
		}
	}
}

func TestHttpBlobApi(t *testing.T) {
	// branches & tags can't start or end with a '/', which is a blessing
	// probably should dump a list of all branches & tags, do a 'startswith'
	// on the incoming string, and if it matches up inside of '/'s, then use that.

	for _, test_case := range []struct {
		BlobName,
		BlobMd5,
		Path string
	}{
		//{starting_hash, md5_of_starting_file, "gitserve.go"}, // Easy case is definitely "no slashes allowed"
		{"tags/0.0.0.0.1", md5_of_gitserve_at_first_tag, "gitserve.go"}, // Let's try it with a human-readable name
		//{"82fcd77642ac584c7debd8709b48d799d7b9fa33", md5_of_nested_testfile, "a/b/c/testfile"},
	} {
		url := path.Join("/blob/", test_case.BlobName, test_case.Path)
		t.Log(url)
		req, err := http.NewRequest("GET", url, nil)
		if err != nil {
			t.Error("Test request failed", err)
		}
		w := httptest.NewRecorder()
		servePath(w, req)
		if w.Code != 200 {
			t.Error(w.Code, w.Body.String())
		}
		output_hash := fmt.Sprintf("%x", md5.Sum([]byte(w.Body.String())))
		if output_hash != test_case.BlobMd5 {
			t.Log(fmt.Sprintf("failed: %q", w.Body.String()))
			t.Error("Output not what we expected- check ", test_case.Path, "\n\nand hashes ", output_hash, " vs ", test_case.BlobMd5)
		}
		t.Log("-=-=-=-==-==-=-=-=-=-=-==-==-=-==-=-")
	}
}
