package bundle

//go:generate sh -c "go run goembed.go -package bundled -var assets assets/header.tmpl assets/footer.tmpl assets/style.css assets/manpage.tmpl assets/manpageerror.tmpl assets/manpagefooterextra.tmpl assets/contents.tmpl assets/pkgindex.tmpl assets/srcpkgindex.tmpl assets/index.tmpl assets/faq.tmpl assets/notfound.tmpl assets/Inconsolata.woff assets/Inconsolata.woff2 assets/opensearch.xml assets/Roboto-Bold.woff assets/Roboto-Bold.woff2 assets/Roboto-Regular.woff assets/Roboto-Regular.woff2 > internal/bundled/GENERATED_bundled.go"
