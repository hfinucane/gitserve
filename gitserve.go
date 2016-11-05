package main

// This Source Code Form is subject to the terms of the Mozilla Public License, v. 2.0. If a copy of the MPL was not distributed with this file, You can obtain one at https://mozilla.org/MPL/2.0/.

import (
	"flag"
	"fmt"
	"net/http"
	"os"
	"os/exec"
	"sort"
	"strings"
)

func stripLeadingSlash(path string) string {
	if len(path) > 1 && path[0] == '/' {
		return path[1:]
	}
	return path
}

func pickLongestRef(url string, refs []string) (string, string, error) {
	// Check if there is a human-readable ref, which may contain slashes, here
	for _, ref := range refs {
		if strings.HasPrefix(url, ref) {
			return ref, stripLeadingSlash(url[len(ref):]), nil
		}
	}
	// For some reason I have it in my head that it's best to test all exact
	// matches before falling back on fuzzy ones
	for _, ref := range refs {
		// XXX: Is resolving branches before tags the right thing to do?
		if strings.HasPrefix("heads/"+url, ref) {
			return ref, stripLeadingSlash(url[len(ref)-len("heads/"):]), nil
		} else if strings.HasPrefix("tags/"+url, ref) {
			return ref, stripLeadingSlash(url[len(ref)-len("tags/"):]), nil
		}
	}
	return "", "", fmt.Errorf("Could not find %q in %q", url, refs)
}

func stripTrailingSlash(input string) (output string) {
	output = input
	if len(input) > 0 && input[len(input)-1] == '/' {
		output = input[:len(input)-1]
	}
	return output
}

func servePath(writer http.ResponseWriter, request *http.Request) {
	refs, err := getRefs()
	if err != nil {
		fmt.Fprint(writer, err)
		writer.WriteHeader(http.StatusInternalServerError)
		return
	}
	sort.Sort(lengthwise(refs))

	stripped_path := stripTrailingSlash(request.URL.Path)

	// Make sure we're in the right place doing the right thing
	pathComponents := strings.Split(stripped_path, "/")
	if pathComponents[0] != "" || pathComponents[1] != "blob" {
		writer.WriteHeader(http.StatusNotFound)
		return
	}

	trimlen := len("/blob/")

	// Pick from human readable refs
	ref, path, err := pickLongestRef(stripped_path[trimlen:], refs)

	// If it isn't a ref, assume it's a hash literal
	if err != nil {
		ref = pathComponents[2]
		path = strings.Join(pathComponents[3:], "/")
	}

	blob, err := getObject(ref, stripped_path, path)

	if err != nil {
		writer.WriteHeader(http.StatusNotFound)
		fmt.Fprint(writer, err)
		return
	}
	fmt.Fprint(writer, string(blob))
}

func main() {
	var gitPath = flag.String("repo", ".", "git repo to serve")
	var address = flag.String("listen", "0.0.0.0", "what address to listen to")
	var port = flag.Int("port", 6504, "port to listen on")
	flag.Parse()

	// Move to the git repo
	err := os.Chdir(*gitPath)
	if err != nil {
		fmt.Println("Could not move to", gitPath, err)
		os.Exit(2)
	}

	// Verify this is is a git directory
	cmd := exec.Command("git", "status")
	err = cmd.Run()
	if err != nil {
		fmt.Println("Git didn't like ", *gitPath, " got ", err)
		os.Exit(3)
	}

	http.HandleFunc("/blob/", servePath)
	fmt.Println(http.ListenAndServe(fmt.Sprintf("%s:%d", *address, *port), nil))
}
