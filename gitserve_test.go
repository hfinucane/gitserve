package main

// This Source Code Form is subject to the terms of the Mozilla Public License, v. 2.0. If a copy of the MPL was not distributed with this file, You can obtain one at https://mozilla.org/MPL/2.0/.

import (
	"crypto/md5"
	"fmt"
	"io/ioutil"
	"net/http"
	"net/http/httptest"
	"path"
	"strings"
	"testing"
)

var startingHash = "2ccc62d64502f9e7f1231c5b228136d3ee0fa72c"
var firstGitserveMD5 = "0566ec561947146909cf40192cda39ec"
var firstTaggedGitserveMD5 = "bc01be1e5c1fbdbe31ac89ae8fb154cd"
var nestedFileMD5 = "d8e8fca2dc0f896fd7cb4cb0031ba249"

func TestDisplayingObject(t *testing.T) {
	firstCommit, err := getObject(startingHash, "prefix", "gitserve.go")

	firstCommitMD5 := fmt.Sprintf("%x", md5.Sum(firstCommit))

	if err != nil {
		t.Error(err)
	}
	if firstCommitMD5 != firstGitserveMD5 {
		t.Errorf("%s came back- not %s\n", firstCommitMD5, firstGitserveMD5)
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
	refs, err := getRefs()
	if err != nil {
		t.Error(err)
	}

	// Check that existing tags exist
	// Check that remotes/origin/master exists
	// This *is* a little dependent on where you get this from,
	// so maybe I should build a little dedicated demo submodule ...
	var rootedTag, zeroethTag, remoteMasterBranch bool = false, false, false
	for _, ref := range refs {
		if ref == "remotes/origin/master" {
			remoteMasterBranch = true
		} else if ref == "tags/0.0.0.0.1" {
			zeroethTag = true
		} else if ref == "tags/rooted/tags/are/tricky" {
			rootedTag = true
		}
	}
	if !rootedTag {
		t.Error("didn't find tags/rooted/tags/are/tricky")
	} else if !zeroethTag {
		t.Error("didn't find tags/0.0.0.0.1")
	} else if !remoteMasterBranch {
		t.Error("didn't find remotes/origin/master- this can be checkout dependent, sorry for the flaky test")
	}
}

func TestDisplayingMissingObject(t *testing.T) {
	firstCommit, err := getObject(startingHash, "prefix", "quack")

	if err == nil {
		t.Error("This should be an error- this is not a legit file")
	}
	if firstCommit != nil {
		t.Errorf("What are you doing returning content here? '%q'", firstCommit)
	}
}

func TestDisplayingBadRoot(t *testing.T) {
	firstCommit, err := getObject("invalid_hash", "prefix", "gitserve.go")

	if err == nil {
		t.Error("This should be an error- this is not a legit hash")
	}
	if firstCommit != nil {
		t.Errorf("What are you doing returning content here? '%q'", firstCommit)
	}
}

func TestPickLongestRef(t *testing.T) {
	for _, testCase := range []struct {
		Path        string
		CorrectRef  string
		CorrectPath string
		Refs        []string
	}{
		{"master/Makefile", "heads/master", "Makefile", []string{"heads/master", "tags/1.7"}},
	} {
		ref, path, err := pickLongestRef(testCase.Path, testCase.Refs)
		if ref != testCase.CorrectRef || path != testCase.CorrectPath {
			t.Log("ref", ref, "path", path)
			t.Errorf("Could not match /blob/%s against ref '%s'", testCase.Path, testCase.CorrectRef)
		} else if err != nil {
			t.Error("Threw an error (%s) inappropriately picking %s out of %q", err, ref, path)
		}
	}

	ref, path, err := pickLongestRef("master/Makefile", []string{"heads/master", "tags/1.7"})
	if ref != "heads/master" || path != "Makefile" {
		t.Log("ref", ref, "path", path)
		t.Error("Could not match /blob/master/Makefile against ref 'master'")
	} else if err != nil {
		t.Error("Threw an error inappropriately picking foo out of ['heads/master', 'tags/1.7']")
	}

	ref, path, err = pickLongestRef("foo", []string{"foo", "bar", "baz"})
	if ref != "foo" || path != "" {
		t.Log("ref", ref, "path", path)
		t.Error("Could not match /blob/foo against ref 'foo'")
	} else if err != nil {
		t.Error("Threw an error inappropriately picking foo out of ['foo','bar','baz']")
	}

	ref, path, err = pickLongestRef("foo/baz.txt", []string{"foo", "bar", "baz"})
	if ref != "foo" || path != "baz.txt" {
		t.Log("ref", ref, "path", path)
		t.Error("Could not match /blob/foo/baz.txt against ref 'foo'")
	} else if err != nil {
		t.Error("Threw an error inappropriately with file name")
	}

	ref, path, err = pickLongestRef("tags/can/have/slashes/baz.txt", []string{"tags/can/have/slashes", "tags/can", "tags"})
	if ref != "tags/can/have/slashes" || path != "baz.txt" {
		t.Log("ref", ref, "path", path)
		t.Error("Could not match /blob/tags/can/have/slashes/baz.txt")
	} else if err != nil {
		t.Error("Threw an error inappropriately with a nested ref")
	}

	ref, path, err = pickLongestRef("do/not/eat/everything/baz.txt", []string{"do", "not", "eat"})
	if ref != "do" || path != "not/eat/everything/baz.txt" {
		t.Log("ref", ref, "path", path)
		t.Error("Could not match /blob/do/not/eat/everything/baz.txt to 'do'")
	} else if err != nil {
		t.Error("Threw an error inappropriately with a non-greedy match")
	}
}

func TestPathRsplit(t *testing.T) {
	for _, testCase := range []struct {
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
		root, branch := pathRsplit(testCase.Path)
		if root != testCase.OutputA {
			t.Error("root", root, "does not match", testCase.OutputA, "from", testCase.Path)
		}
		if branch != testCase.OutputB {
			t.Error("branch", branch, "does not match", testCase.OutputB, "from", testCase.Path)
		}
	}
}

func TestHttpTreeApi(t *testing.T) {
	// If you go to http://server:port/blob/master, you might hope to get a file
	// listing instead of a 404
	for _, tc := range []struct {
		Blob, Path      string
		ExpectedEntries []string
	}{
		// XXX FIXME Check branch name root too
		{"rooted/tags/may/confuse", "/", []string{"gitserve.go", "gitserve_test.go"}},
		{"2ccc6", "/", []string{"gitserve.go"}},
		{"82fcd77642", "/a", []string{"b"}},
		{"82fcd77642", "/a/", []string{"blob/82fcd77642/a/b"}},
		{"82fcd77642", "/a/b", []string{"c/"}},
		{"82fcd77642", "/a/b/c/", []string{"testfile"}},
	} {
		req, err := http.NewRequest("GET", path.Join("/blob/", tc.Blob, tc.Path), nil)
		if err != nil {
			t.Fatal("Test request failed", err)
		}
		w := httptest.NewRecorder()
		servePath(w, req)

		listing := w.Body.String()
		t.Log(path.Join("/blob/", tc.Blob, tc.Path))
		for _, entry := range tc.ExpectedEntries {
			if !strings.Contains(listing, entry) {
				t.Fatal("Output not what we expected- missing ", entry, " from ", tc.Path, " @ ", tc.Blob, "got:\n", textSample(listing))
			}
		}
	}
}

func textSample(incoming string) string {
	if len(incoming) > 200 {
		return incoming[0:200]
	}
	return incoming
}

func TestHttpBlobApi(t *testing.T) {
	// branches & tags can't start or end with a '/', which is a blessing
	// probably should dump a list of all branches & tags, do a 'startswith'
	// on the incoming string, and if it matches up inside of '/'s, then use that.

	for _, tc := range []struct {
		BlobName,
		BlobMd5,
		Path string
	}{
		{startingHash, firstGitserveMD5, "gitserve.go"},           // Easy case is definitely "no slashes allowed"
		{"tags/0.0.0.0.1", firstTaggedGitserveMD5, "gitserve.go"}, // Let's try it with a human-readable name
		{"82fcd77642ac584c7debd8709b48d799d7b9fa33", nestedFileMD5, "a/b/c/testfile"},
	} {
		url := path.Join("/blob/", tc.BlobName, tc.Path)
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
		outputHash := fmt.Sprintf("%x", md5.Sum([]byte(w.Body.String())))
		if outputHash != tc.BlobMd5 {
			t.Log(fmt.Sprintf("failed: %q", w.Body.String()))
			ioutil.WriteFile("/tmp/failed", []byte(w.Body.String()), 0644)
			t.Error("Output not what we expected- check ", tc.Path, "\n\nand hashes ", outputHash, " vs ", tc.BlobMd5, " bad output sample:\n", textSample(w.Body.String()))
		}
		t.Log("-=-=-=-==-==-=-=-=-=-=-==-==-=-==-=-")
	}
}
