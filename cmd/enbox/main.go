package main

import (
	"flag"
	"fmt"
	"os"
	"os/exec"
	"os/user"
	"path/filepath"
)

func run() error {
	// flags
	var dir, entryPoint string
	var offline bool
	flag.StringVar(&dir, "dir", "", "writeable directory")
	flag.StringVar(&entryPoint, "exec", "sh", "entry point")
	flag.BoolVar(&offline, "offline", false, "disable internet")
	flag.Parse()

	home, err := os.UserHomeDir()
	if err != nil {
		return err
	}
	local := filepath.Join(home, ".local")
	cache, err := os.UserCacheDir()
	if err != nil {
		return err
	}

	// default to cwd
	if dir == "" {
		var err error
		dir, err = os.Getwd()
		if err != nil {
			return err
		}
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
		"--bind", dir, dir,
		"--bind", cache, cache,
		"--bind", local, local,
		}

	if offline {
		args = append(args, "--unshare-net")
	}

	args = append(args, []string {
		"--chdir", dir,
		entryPoint,
	}...)

	cmd := exec.Command(args[0], args[1:]...)
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	cmd.Run()

	return nil
}

func main() {
	err := run()
	if err != nil {
		fmt.Fprintf(os.Stderr, "%s\n", err)
		os.Exit(1)
	}
}
