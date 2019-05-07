# debiman

[![Build Status](https://travis-ci.org/Debian/debiman.svg?branch=master)](https://travis-ci.org/Debian/debiman)
[![Go Report Card](https://goreportcard.com/badge/github.com/Debian/debiman)](https://goreportcard.com/report/github.com/Debian/debiman)

<img src="https://debian.github.io/debiman/debiman-logo.svg" width="300" height="280" align="right" alt="debiman logo">

## Goals

debiman makes (Debian) manpages accessible in a web browser. Its goals are, in order:

1. **completeness**: all manpages in Debian should be available.
2. **visually appealing** and **convenient**: reading manpages should be fun, convenience features (e.g. permalinks, URL redirects, easy navigation) should be available
3. **speed**: manpages should be quick to load, new manpages should be quickly ingested, the program should run quickly for pleasant development

Currently, there is one known bug with regards to completeness ([#12](https://github.com/Debian/debiman/issues/12)).

With regards to speed, debiman can process all manpages of Debian unstable in **less than 10 minutes** on a modern machine. Incremental updates complete in **less than 15 seconds**. For more details, see [PERFORMANCE.md](https://github.com/Debian/debiman/blob/master/PERFORMANCE.md).

## Prerequisites

* mandoc
* apt-cacher-ng running on localhost:3142
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
2. Man pages and auxiliary files (e.g. content fragment files which are included by a number of manpages) are extracted from the identified Debian packages.
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
go get -u github.com/Debian/debiman/cmd/...
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

For https://manpages.debian.org, we run:

```
flock /srv/manpages.debian.org/debiman/exclusive.lock \
nice -n 5 \
ionice -n 7 \
debiman \
  -sync_codenames=oldstable,oldstable-backports,stable,stable-backports \
  -sync_suites=testing,unstable,experimental \
  -serving_dir=/srv/manpages.debian.org/www \
  -local_mirror=/srv/mirrors/debian
```
    
…resulting in the directories wheezy/, wheezy-backports/, jessie/, jessie-backports/, testing/, unstable/ and experimental/ (respectively).

Note that you will *NOT* need to change this command line when a new version of Debian is released.

When interrupted, you can just run debiman again with the same options. It will resume where it left off.

If for some reason you notice corruption or other mistakes in some manpages, just delete the directory in which they are placed, then re-run debiman to download and re-process these pages from scratch.

It is safe to run debiman while you are serving from `-serving_dir`. debiman will swap files atomically using [rename(2)](https://manpages.debian.org/rename(2)).

## Customization

You can copy the `assets/` directory, modify its contents and start
debiman with `-inject_assets` pointed to your directory. Any files whose
name does not end in .tmpl are treated as static files and will be
placed in -serving_dir (compressed and uncompressed).

There are a few requirements for the templates, so that debiman can
re-use rendered manpages (for symlinked manpages):

1. In `assets/manpage.tmpl` and `assets/manpageerror.tmpl`, the string `<a
   class="toclink"` is used to find table of content links.
2. `</div>\n</div>\n<div id="footer">` is used to delimit the mandoc output
   from the rest of the page.

## interesting test cases

[crontab(5)](https://manpages.debian.org/crontab(5)) is present in multiple Debian versions, multiple languages, multiple sections and multiple conflicting packages. Hence, it showcases all debiman features.

[w3m(1)](https://manpages.debian.org/w3m(1)) has a Japanese translation which is only present in UTF-8 starting with Debian jessie. It also has a German translation starting with Debian stretch.

[qelectrotech(1)](https://manpages.debian.org/qelectrotech(1)) has a French translation in 3 different encodings (none specified, ISO8859-1, UTF-8).

[mysqld(8)](https://manpages.debian.org/mysqld(8)) is present in two conflicting packages: `mariadb-server-core-10.0` and `mysql-server-core-5.6`.

## recommended reading

https://wiki.debian.org/RepositoryFormat

## URLs

The URL schema which debiman uses is `(<suite>/)(<binarypkg/>)<name>(.<section>(.<lang>))`. Any part aside from `name` can be omitted; here are a few examples:

Without suite and binary package:

1. https://manpages.debian.org/i3
2. https://manpages.debian.org/i3.fr
3. https://manpages.debian.org/i3.1
4. https://manpages.debian.org/i3.1.fr

With binary package:

1. https://manpages.debian.org/i3-wm/i3
2. https://manpages.debian.org/i3-wm/i3.fr
3. https://manpages.debian.org/i3-wm/i3.1
4. https://manpages.debian.org/i3-wm/i3.1.fr

With suite:

1. https://manpages.debian.org/testing/i3
2. https://manpages.debian.org/testing/i3.fr
3. https://manpages.debian.org/testing/i3.1
4. https://manpages.debian.org/testing/i3.1.fr

With suite and binary package:

1. https://manpages.debian.org/testing/i3-wm/i3
2. https://manpages.debian.org/testing/i3-wm/i3.fr
3. https://manpages.debian.org/testing/i3-wm/i3.1
4. https://manpages.debian.org/testing/i3-wm/i3.1.fr
