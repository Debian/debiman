// copied from https://github.com/dsymonds/goembed/ with pull requests applied

// +build ignore

// goembed generates a Go source file from an input file.
package main

import (
	"bytes"
	"compress/gzip"
	"flag"
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"os"
	"text/template"
)

var (
	packageFlag = flag.String("package", "", "Go package name")
	varFlag     = flag.String("var", "", "Go var name")
	gzipFlag    = flag.Bool("gzip", false, "Whether to gzip contents")
)

func main() {
	flag.Parse()

	fmt.Printf("package %s\n\n", *packageFlag)

	if flag.NArg() > 0 {
		fmt.Println("// Table of contents")
		fmt.Printf("var %v = map[string]string{\n", *varFlag)
		for i, filename := range flag.Args() {
			fmt.Printf("\t%#v: %s_%d,\n", filename, *varFlag, i)
		}
		fmt.Println("}")

		// Using a separate variable for each []byte, instead of
		// combining them into a single map literal, enables a storage
		// optimization: the compiler places the data directly in the
		// program's noptrdata section instead of the heap.
		for i, filename := range flag.Args() {
			if err := oneVar(fmt.Sprintf("%s_%d", *varFlag, i), filename); err != nil {
				log.Fatal(err)
			}
		}
	} else {
		if err := oneVarReader(*varFlag, os.Stdin); err != nil {
			log.Fatal(err)
		}
	}
}

func oneVar(varName, filename string) error {
	f, err := os.Open(filename)
	if err != nil {
		return err
	}
	defer f.Close()
	return oneVarReader(varName, f)
}

func oneVarReader(varName string, r io.Reader) error {
	raw, err := ioutil.ReadAll(r)
	if err != nil {
		return err
	}

	// Generate []byte(<big string constant>) instead of []byte{<list of byte values>}.
	// The latter causes a memory explosion in the compiler (60 MB of input chews over 9 GB RAM).
	// Doing a string conversion avoids some of that, but incurs a slight startup cost.
	if !*gzipFlag {
		fmt.Printf(`var %s = "`, varName)
	} else {
		var buf bytes.Buffer
		gzw, _ := gzip.NewWriterLevel(&buf, gzip.BestCompression)
		if _, err := gzw.Write(raw); err != nil {
			return err
		}
		if err := gzw.Close(); err != nil {
			return err
		}
		gz := buf.Bytes()

		if err := gzipPrologue.Execute(os.Stdout, varName); err != nil {
			return err
		}
		fmt.Printf("var %s string // set in init\n\n", varName)
		fmt.Printf(`var %s_gzip = "`, varName)
		raw = gz
	}

	for _, b := range raw {
		fmt.Printf("\\x%02x", b)
	}
	fmt.Println(`"`)
	return nil
}

var gzipPrologue = template.Must(template.New("").Parse(`
import (
	"bytes"
	"compress/gzip"
	"io/ioutil"
)
func init() {
	r, err := gzip.NewReader(bytes.NewReader({{.}}_gzip))
	if err != nil {
		panic(err)
	}
	defer r.Close()
	{{.}}, err = ioutil.ReadAll(r)
	if err != nil {
		panic(err)
	}
}
`))
