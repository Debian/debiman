# debiman

[![Build Status](https://travis-ci.org/Debian/debiman.svg?branch=master)](https://travis-ci.org/Debian/debiman)
[![Go Report Card](https://goreportcard.com/badge/github.com/Debian/debiman)](https://goreportcard.com/report/github.com/Debian/debiman)

## Prerequisites

* mandoc
* a number of Go packages (which `go get` will automatically get for you, see below)
    * pault.ag/go/debian
    * pault.ag/go/archive
    * github.com/golang/protobuf/proto
    * golang.org/x/crypto/openpgp
    * golang.org/x/net/html
    * golang.org/x/sync/errgroup
    * golang.org/x/text

## Architecture overview

debiman works in 4 stages:

1. All Debian packages of all architectures of the specified suites are discovered. The following optimizations are used to reduce the number of packages, and hence the input size/required bandwidth:
    1. packages which do not own any files in /usr/share/man (as per the Contents-<arch> archive files) are skipped.
    2. each package is downloaded only for 1 of its architectures, as manpages are architecture-independent.
2. Man pages and auxilliary files (e.g. content fragment files which are included by a number of manpages) are extracted from the identified Debian packages.
3. All man pages are rendered into an HTML representation using mandoc(1).
4. An index file for debiman-auxserver (which serves redirects) is written.

Each stage runs concurrently (e.g. Contents and Packages files are
inspected concurrently), but only one stage runs at a time,
e.g. extraction needs to complete before rendering can start.

## Development quick start

### Set up Go

If this is the first time you’re using Go, run:
```
sudo apt install golang-go
export GOPATH=~/go
```

### Install debiman

To download, compile and install debiman to `$GOPATH/bin`, run:
```
go get -u github.com/Debian/debiman
```

### Run debiman

To synchronize Debian testing to ~/man and render a handful of packages, run:
```
$GOPATH/bin/debiman -serving_dir=~/man -only_render_pkgs=qelectrotech,i3-wm,cron
```

### Test the output

To serve manpages from ~/man on localhost:8089, run:
```
$GOPATH/bin/debiman-minisrv -serving_dir=~/man
```

Note that for a production setup, you should not use debiman-minisrv. Instead,
refer to the web server example configuration files in example/.

### Recompile debiman

To update your debiman installation after making changes to the HTML
templates or code in `$GOPATH/src/github.com/Debian/debiman`, run:
```
go generate github.com/Debian/debiman/...
go install github.com/Debian/debiman/...
```

## Synchronizing

For manpages.debian.org, we run:

  debiman \
    -sync_codenames=oldstable,oldstable-backports,stable,stable-backports \
    -sync_suites=testing,unstable
    
…resulting in the directories wheezy/, wheezy-backports/, jessie/, jessie-backports/, testing/ and unstable/ (respectively).

Note that you will *NOT* need to change this command line when a new version of Debian is released.

When interrupted, you can just run debiman again with the same options. It will resume where it left off.

If for some reason you notice corruption or other mistakes in some manpages, just delete the directory in which they are placed, then re-run debiman to download and re-process these pages from scratch.

It is safe to run debiman while you are serving from -serving_dir. debiman will swap files atomically using rename(2).

TODO: add numbers for how long synchronization typically takes from scratch and incrementally, also how much network traffic it will cause

## Customization

You can copy the assets/ directory, modify its contents and start
debiman with -inject_assets pointed to your directory. Any files whose
name does not end in .tmpl are treated as static files and will be
placed in -serving_dir (compressed and uncompressed).

There are a few requirements for the templates, so that debiman can
re-use rendered manpages (for symlinked manpages):

1. In assets/manpage.tmpl and assets/manpageerror.tmpl, the string `<a
   class="toclink"` is used to find table of content links.
2. `</div>\n<div id="footer">` is used to delimit the mandoc output
   from the rest of the page.

## interesting test cases

crontab(5) is present in multiple Debian versions, multiple languages, multiple sections and multiple conflicting packages. Hence, it showcases all debiman features.

w3m(1) has a japanese translation which is only present in UTF-8 starting with Debian jessie. It also has a German translation starting with Debian stretch.

qelectrotech(1) has a french translation in 3 different encodings (none specified, ISO8859-1, UTF-8).

mysqld(8) is present in two conflicting packages: mariadb-server-core-10.0 and mysql-server-core-5.6.

## recommended reading

https://wiki.debian.org/RepositoryFormat

## URLs

<language>
content-type (one of text/html, text/plain, <raw>)
compression (gzip, brötli) — NOT indicated in the URL. can $server statically serve brötli? will $server uncompress compressed files?

debian suite redirect (oldstable→wheezy, stable→jessie, TODO: backports)

examples:
/testing/i3-wm/i3.1 → redirect: /testing/i3-wm/i3.1.fr.html
/testing/i3-wm/i3.1 → redirect: /testing/i3-wm/i3.1.fr.txt

fallback: static redirect to .en.html
