package main

import (
	"crypto/md5"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
)

var starting_hash string = "b1dc9af6f6d8d7ce5d5a0fff1cee73ae9d44c7bb"
var md5_of_starting_file string = "0566ec561947146909cf40192cda39ec"

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
	// `git show-ref --abbrev` is going to be the source of this data
	// take off the leading 'refs/', and use the rest as the list of object sources
	// means the data source can be fed via `git fetch` (shows up under 'remotes')
	// or `git push` (data shows up under 'heads'). As long as the user is consistent,
	// and there's an interface to list refs, everything should be pretty reasonable,
	// though it's less concise, than, say, github's url structure (they can cheat &
	// require `git push`)
	var refs []string
	refs, err := get_refs()

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
		t.Error("didn't find rooted/tags/are/tricky")
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

func TestHttpBlobApi(t *testing.T) {
	// branches & tags can't start or end with a '/', which is a blessing
	// probably should dump a list of all branches & tags, do a 'startswith'
	// on the incoming string, and if it matches up inside of '/'s, then use that.

	// Easy case is definitely "no slashes allowed"
	req, err := http.NewRequest("GET", "http://example.com/blob/"+starting_hash+"/gitserve.go", nil)
	if err != nil {
		t.Error("Test request failed", err)
	}
	w := httptest.NewRecorder()
	servePath(w, req)
	output_hash := fmt.Sprintf("%x", md5.Sum([]byte(w.Body.String())))
	if output_hash != md5_of_starting_file {
		t.Error("Output not what we expected- check /tmp/dat1\n\nand hashes ", output_hash, " vs ", md5_of_starting_file)
	}
}
