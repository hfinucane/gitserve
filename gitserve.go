package main

// This Source Code Form is subject to the terms of the Mozilla Public License, v. 2.0. If a copy of the MPL was not distributed with this file, You can obtain one at https://mozilla.org/MPL/2.0/.

import (
	"errors"
	"flag"
	"fmt"
	"net/http"
	"os"
	"os/exec"
	"sort"
	"strings"
)

func stripLeadingSlash(path string) string {
	fmt.Println("Stripping leading slash from ", path)
	if len(path) > 1 && path[0] == '/' {
		fmt.Println(path[1:])
		return path[1:]
	}
	return path
}

func pick_longest_ref(url string, refs []string) (string, string, error) {
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
	return "", "", errors.New(fmt.Sprintf("Could not find %q in %q", url, refs))
}

func servePath(writer http.ResponseWriter, request *http.Request) {
	refs, err := get_refs()
	if err != nil {
		fmt.Fprint(writer, err)
		writer.WriteHeader(http.StatusInternalServerError)
		return
	}
	sort.Sort(Lengthwise(refs))

	// Make sure we're in the right place doing the right thing
	path_components := strings.Split(request.URL.Path, "/")
	fmt.Println("Path components", path_components)
	if path_components[0] != "" || path_components[1] != "blob" {
		writer.WriteHeader(http.StatusNotFound)
		return
	}

	trimlen := len("/blob/")

	fmt.Printf("all refs: %q\n", refs)

	// Pick from human readable refs
	ref, path, err := pick_longest_ref(request.URL.Path[trimlen:], refs)

	// If it isn't a ref, assume it's a hash literal
	if err != nil {
		fmt.Println("Assuming we got a hash literal- ", err)
		ref = path_components[2]
		path = strings.Join(path_components[3:], "/")
	}

	fmt.Println("ref ", ref)

	blob, err := get_object(ref, request.URL.Path, path)

	if err != nil {
		writer.WriteHeader(http.StatusNotFound)
		fmt.Fprint(writer, err)
		return
	}
	fmt.Fprint(writer, string(blob))
}

func main() {
	var git_path *string = flag.String("repo", ".", "git repo to serve")
	var address *string = flag.String("listen", "0.0.0.0", "what address to listen to")
	var port *int = flag.Int("port", 6504, "port to listen on")
	flag.Parse()

	// Move to the git repo
	err := os.Chdir(*git_path)
	if err != nil {
		fmt.Println(err)
		os.Exit(2)
	}

	// Verify this is is a git directory
	cmd := exec.Command("git", "status")
	err = cmd.Run()
	if err != nil {
		fmt.Println("Git didn't like ", *git_path, " got ", err)
		os.Exit(3)
	}

	http.HandleFunc("/blob/", servePath)
	http.ListenAndServe(fmt.Sprintf("%s:%d", *address, *port), nil)
}
