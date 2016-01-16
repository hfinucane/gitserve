package main

import (
	"crypto/md5"
	"fmt"
	"io/ioutil"
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
	ioutil.WriteFile("/tmp/dat1", []byte(w.Body.String()), 0644)
	if output_hash != md5_of_starting_file {
		t.Error("Output not what we expected- check /tmp/dat1\n\nand hashes ", output_hash, " vs ", md5_of_starting_file)
	}
	// assert that md5.Sum(w.Body.String()) == md5_of_starting_file
}
