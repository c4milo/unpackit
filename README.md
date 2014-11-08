# UnzipIt
[![GoDoc](https://godoc.org/github.com/c4milo/unzipit?status.svg)](https://godoc.org/github.com/c4milo/unzipit)
[![Build Status](https://travis-ci.org/c4milo/unzipit.svg?branch=master)](https://travis-ci.org/c4milo/unzipit)

This Go library allows you to easily unpack the following files:

* tar.gz
* tar.bzip2
* zip 
* tar

There are not CGO involved nor hard dependencies of any type.

## Usage
Unpack a file:
```
    file, err := os.Open(test.filepath)
    ok(t, err)
    defer file.Close()

    destPath, err := unzipit.Unpack(file, tempDir)
```

Unpack a stream (such as a http.Response):
```
    res, err := http.Get(url)
    destPath, err := unzipit.UnpackStream(res.Body, tempDir)
```

