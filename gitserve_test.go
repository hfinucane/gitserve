package main

import (
	"crypto/md5"
	"fmt"
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
