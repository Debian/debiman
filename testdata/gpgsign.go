//go:build ignore

// Program gpgsign clearsigns testdata/tinymirror with a temporary key.
package main

import (
	"bytes"
	"log"
	"os"

	"github.com/ProtonMail/go-crypto/openpgp"
	"github.com/ProtonMail/go-crypto/openpgp/clearsign"
)

func signatureValid(release []byte) bool {
	f, err := os.Open("testdata/tinymirror/keyring.gpg")
	if err != nil {
		return false
	}
	defer f.Close()
	keyring, err := openpgp.ReadKeyRing(f)
	if err != nil {
		return false
	}
	inRelease, err := os.ReadFile("testdata/tinymirror/dists/testing/InRelease")
	if err != nil {
		return false
	}
	block, _ := clearsign.Decode(inRelease)
	if block == nil {
		return false
	}
	if _, err := block.VerifySignature(keyring, nil); err != nil {
		return false
	}
	return bytes.Equal(block.Plaintext, release)
}

func gpgsign() error {
	release, err := os.ReadFile("testdata/tinymirror/dists/testing/Release")
	if err != nil {
		return err
	}
	// Check if InRelease is the signed version of Release.
	if signatureValid(release) {
		// Make 'go generate' a no-op when a valid signature is already present.
		return nil
	}

	// Create a signing authority.
	const comment = ""
	const email = ""
	authority, err := openpgp.NewEntity("debiman testdata", comment, email, nil)
	if err != nil {
		return err
	}

	// Export the public key to a keyring file.
	var keyring bytes.Buffer
	if err := authority.Serialize(&keyring); err != nil {
		return err
	}
	if err := os.WriteFile("testdata/tinymirror/keyring.gpg", keyring.Bytes(), 0644); err != nil {
		return err
	}

	// Clearsign the Release file into InRelease.
	var inRelease bytes.Buffer
	w, err := clearsign.Encode(&inRelease, authority.PrivateKey, nil)
	if err != nil {
		return err
	}
	if _, err := w.Write(release); err != nil {
		return err
	}
	if err := w.Close(); err != nil {
		return err
	}
	if err := os.WriteFile("testdata/tinymirror/dists/testing/InRelease", inRelease.Bytes(), 0644); err != nil {
		return err
	}

	return nil
}

func main() {
	if err := gpgsign(); err != nil {
		log.Fatal(err)
	}
}
