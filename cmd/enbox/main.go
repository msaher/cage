package main

import (
	"errors"
	"flag"
	"fmt"
	"os"
	"os/exec"
	"os/user"
	"path/filepath"
)

var (
	cwd string
)

type Dirs []string

func (d *Dirs) String() string {
	return fmt.Sprint([]string(*d))
}

func (d *Dirs) Set(v string) error {
	if v == "." {
		v = cwd
	}

	if v != "" {
		*d = append(*d, v)
	}
	return nil
}

func run() error {
	// ensure bubblewrap is installed
	_, err := exec.LookPath("bwrap")
	if err != nil {
		return err
	}

	cwd, err = os.Getwd()
	if err != nil {
		return err
	}

	// flags
	var rwDirs Dirs
	var roDirs Dirs
	var chdir string
	var offline bool
	flag.Var(&rwDirs, "rw", "add read-write directory (can repeat)")
	flag.Var(&roDirs, "ro", "add read-only directory (can repeat)")
	flag.StringVar(&chdir, "chdir", "", "directory to change into")
	flag.BoolVar(&offline, "offline", false, "no network access")
	flag.Parse()

	entryPoint := flag.Args()

	if chdir == "." {
		chdir = cwd
	}

	home, err := os.UserHomeDir()
	if err != nil {
		return err
	}
	local := filepath.Join(home, ".local")
	cache, err := os.UserCacheDir()
	if err != nil {
		return err
	}

	u, err := user.Current()
	if err != nil {
		return err
	}
	uid := u.Uid
	runtimeDir := filepath.Join("/run", "user", uid)

	// common stuff we need to expose if we later want to make a
	// sandbox that limits whats readable from /
	// like a strict-mode or something
	// "--ro-bind", "/usr", "/usr",
	// "--ro-bind", "/bin", "/bin",
	// "--ro-bind", "/lib", "/lib",
	// "--ro-bind", "/lib64", "/lib64",
	// "--ro-bind", "/etc/resolv.conf", "/etc/resolv.conf", // internet
	// "--ro-bind", "/etc/passwd", "/etc/passwd", // whoami
	// "--ro-bind", "/etc/group", "/etc/group", // whoami
	// "--dir", home, idk
	// also config
	args := []string{
		"bwrap",

		// make entire host read-only
		"--ro-bind", "/", "/",

		// expose runtime
		"--bind", runtimeDir, runtimeDir,

		// commonly needed
		"--tmpfs", "/tmp",
		"--proc", "/proc",
		"--dev", "/dev",

		// directories you can write to
		"--bind", cache, cache,
		"--bind", local, local,
		}


	for _, d := range roDirs {
		args = append(args, "--ro-bind", d, d)
	}

	for _, d := range rwDirs {
		args = append(args, "--bind", d, d)
	}

	if chdir != "" {
		args = append(args, "--chdir", chdir)
	} else if len(rwDirs) > 0 {
		d := rwDirs[0]
		if d == "." {
			d = cwd
		}
		args = append(args, "--chdir", d)
	}

	if offline {
		args = append(args, "--unshare-net")
	}

	args = append(args, "--")
	if len(entryPoint) == 0 {
		entryPoint = []string{"sh"}
	}
	args = append(args, entryPoint...)

	cmd := exec.Command(args[0], args[1:]...)
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	return cmd.Run()
}

func main() {
	err := run()
	if exitErr, ok := errors.AsType[*exec.ExitError](err); ok {
		// match process exit
		code := exitErr.ExitCode()
		if code > 0 {
			os.Exit(code)
		}
		os.Exit(1)
	} else if _, ok := errors.AsType[*exec.Error](err); ok {
		fmt.Fprintf(os.Stderr, "error: can't find bwrap. Please ensure its installed and in $PATH\n")
		os.Exit(1)
	} else if err != nil {
		fmt.Fprintf(os.Stderr, "%s\n", err)
		os.Exit(1)
	}
}
