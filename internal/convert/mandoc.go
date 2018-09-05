package convert

import (
	"bytes"
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"net"
	"os"
	"os/exec"
	"strings"
	"syscall"

	"golang.org/x/sync/errgroup"
)

// Process starts a mandoc process to convert manpages to HTML.
type Process struct {
	mandocConn    *net.UnixConn
	mandocProcess *os.Process
	stopWait      chan bool
}

func NewProcess() (*Process, error) {
	p := &Process{}
	return p, p.initMandoc()
}

func (p *Process) Kill() error {
	if p.mandocProcess == nil {
		return nil
	}
	p.stopWait <- true
	return p.mandocProcess.Kill()
}

func (p *Process) initMandoc() error {
	pair, err := syscall.Socketpair(syscall.AF_UNIX, syscall.SOCK_STREAM, 0)
	if err != nil {
		return err
	}

	// Use pair[0] in the parent process
	syscall.CloseOnExec(pair[0])
	f := os.NewFile(uintptr(pair[0]), "")
	fc, err := net.FileConn(f)
	if err != nil {
		return err
	}
	conn := fc.(*net.UnixConn)

	path, err := exec.LookPath("mandocd")
	if err != nil {
		if ee, ok := err.(*exec.Error); ok && ee.Err == exec.ErrNotFound {
			log.Printf("mandocd not found, falling back to fork+exec for each manpage")
			return nil
		}
		return err
	}

	cmd := exec.Command(path, "-Thtml", "3") // Go dup2()s ExtraFiles to 3 and onwards
	cmd.ExtraFiles = []*os.File{os.NewFile(uintptr(pair[1]), "")}
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Start(); err != nil {
		return err
	}

	p.stopWait = make(chan bool)
	go func() {
		wait := make(chan error, 1)
		go func() {
			wait <- cmd.Wait()
		}()
		select {
		case <-p.stopWait:
			return
		case err := <-wait:
			log.Fatalf("mandoc unexpectedly exited: %v", err)
		}
	}()

	p.mandocProcess = cmd.Process
	p.mandocConn = conn
	return nil
}

func (p *Process) mandoc(r io.Reader) (stdout string, stderr string, err error) {
	if p.mandocConn != nil {
		stdout, stderr, err = p.mandocUnix(r)
	} else {
		stdout, stderr, err = p.mandocFork(r)
	}
	// TODO(later): once a new-enough version of mandoc is in Debian,
	// get rid of this compatibility code by changing our CSS to not
	// rely on the mandoc class at all anymore.
	if err == nil && !strings.HasPrefix(stdout, `<div class="mandoc">`) {
		stdout = `<div class="mandoc">
` + stdout + `</div>
`
	}
	return stdout, stderr, err
}

func (p *Process) mandocFork(r io.Reader) (stdout string, stderr string, err error) {
	var stdoutb, stderrb bytes.Buffer
	cmd := exec.Command("mandoc", "-Ofragment", "-Thtml")
	cmd.Stdin = r
	cmd.Stdout = &stdoutb
	cmd.Stderr = &stderrb
	if err := cmd.Run(); err != nil {
		return "", "", fmt.Errorf("%v, stderr: %s", err, stderrb.String())
	}
	return stdoutb.String(), stderrb.String(), nil
}

func (p *Process) mandocUnix(r io.Reader) (stdout string, stderr string, err error) {
	manr, manw, err := os.Pipe()
	if err != nil {
		return "", "", err
	}
	defer manr.Close()
	defer manw.Close()

	outr, outw, err := os.Pipe()
	if err != nil {
		return "", "", err
	}
	defer outr.Close()
	defer outw.Close()

	errr, errw, err := os.Pipe()
	if err != nil {
		return "", "", err
	}
	defer errr.Close()
	defer errw.Close()

	scm := syscall.UnixRights(int(manr.Fd()), int(outw.Fd()), int(errw.Fd()))
	if _, _, err := p.mandocConn.WriteMsgUnix(nil, scm, nil); err != nil {
		return "", "", err
	}
	manr.Close()
	outw.Close()
	errw.Close()

	var eg errgroup.Group

	eg.Go(func() error {
		if _, err := io.Copy(manw, r); err != nil {
			return err
		}
		return manw.Close()
	})

	var stdoutb, stderrb []byte

	eg.Go(func() error {
		var err error
		stdoutb, err = ioutil.ReadAll(outr)
		return err
	})

	eg.Go(func() error {
		var err error
		stderrb, err = ioutil.ReadAll(errr)
		return err
	})

	return string(stdoutb), string(stderrb), eg.Wait()
}
