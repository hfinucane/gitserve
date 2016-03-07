# gitserve
## a simple git => http daemon

Serves a single git repo over HTTP. URLs deliberately built to resemble a
cut-down version of github's. Depends on git proper being installed to work.

```
$ ./gitserve --help
Usage of ./gogitserve:
  -listen string
        what address to listen to (default "0.0.0.0")
  -port int
        port to listen on (default 6504)
  -repo string
        git repo to serve (default ".")
```

Simple usage would look like

```
curl http://localhost:6504/blob/master/README.md
```


## Errata

Code is under the [MPL 2.0](https://www.mozilla.org/en-US/MPL/2.0/)

[![Build Status](https://drone.io/github.com/hfinucane/gitserve/status.png)](https://drone.io/github.com/hfinucane/gitserve/latest)
