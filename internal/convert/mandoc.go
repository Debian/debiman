package convert

import (
	"bytes"
	"io"
	"io/ioutil"
	"log"
	"net"
	"os"
	"os/exec"
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
	// TODO: get mandoc version, error if mandoc is not installed

	// TODO: remove once mandoc patch landed upstream
	return nil

	l, err := net.ListenUnix("unix", &net.UnixAddr{Net: "unix"})
	if err != nil {
		return err
	}
	f, err := l.File()
	if err != nil {
		return err
	}

	cmd := exec.Command("mandoc", "-Thtml", "-Ofragment", "-u", "/invalid")
	cmd.ExtraFiles = []*os.File{f}
	cmd.Env = []string{"MANDOC_UNIX_SOCKFD=3"} // go dup2()s each file in ExtraFiles
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

	conn, err := net.DialUnix("unix", nil, l.Addr().(*net.UnixAddr))
	if err != nil {
		return err
	}

	p.mandocConn = conn
	return nil
}

func (p *Process) mandoc(r io.Reader) (stdout string, stderr string, err error) {
	if p.mandocConn != nil {
		return p.mandocUnix(r)
	} else {
		return p.mandocFork(r)
	}
}

func (p *Process) mandocFork(r io.Reader) (stdout string, stderr string, err error) {
	var stdoutb, stderrb bytes.Buffer
	cmd := exec.Command("mandoc", "-Ofragment", "-Thtml")
	cmd.Stdin = r
	cmd.Stdout = &stdoutb
	cmd.Stderr = &stderrb
	if err := cmd.Run(); err != nil {
		return "", "", err
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
