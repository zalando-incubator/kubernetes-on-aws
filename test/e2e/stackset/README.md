## stackset-e2e-tests dependency

This folder contains the `go.mod` file for importing and building the
stackset-controller e2e tests.

If you're updating `stackset-controller` you should also update the version
`go.mod` by running the following command:

```
go get -u github.com/zalando-incubator/stackset-controller
```

The actual build happens from running `make` one level above.
