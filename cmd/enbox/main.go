package main

import (
	"errors"
	"flag"
	"fmt"
	"os"
	"os/exec"
	"os/user"
	"path/filepath"
	"strings"
)

var (
	cwd string
)

type Dirs []string
type Envs []string

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

func (e *Envs) String() string {
	return fmt.Sprint([]string(*e))
}

func (e *Envs) Set(v string) error {
	if v != "" {
		*e = append(*e, v)
	}
	return nil
}


func run() error {
	// ensure bubblewrap is installed
	_, err := exec.LookPath("bwrap")
	if err != nil {
		return err
	}

	// type Dirs needs cwd
	cwd, err = os.Getwd()
	if err != nil {
		return err
	}

	var rwDirs Dirs
	var roDirs Dirs
	var chdir string
	var offline bool
	var print bool
	var clearEnv, inheritAllEnv bool
	var envs Envs
	var wantConfig, wantCache, wantDataDir bool
	flag.Var(&rwDirs, "rw", "add read-write directory (can repeat)")
	flag.Var(&roDirs, "ro", "add read-only directory (can repeat)")
	flag.StringVar(&chdir, "chdir", "", "directory to change into")
	flag.BoolVar(&offline, "offline", false, "no network access")
	flag.BoolVar(&print, "print", false, "print bwrap command; dont run anything")
	flag.BoolVar(&clearEnv, "clearenv", false, "clear environment variables")
	flag.BoolVar(&inheritAllEnv, "inherit-all-env", false, "inherit all environments variables")
	flag.BoolVar(&wantConfig, "ro-config", false, "alias for -ro $XDG_CONFIG_HOME (fallback to ~/.config)")
	flag.BoolVar(&wantCache, "rw-cache", false, "alias for -rw $XDG_CACHE_HOME (fallback to ~/.cache)")
	flag.BoolVar(&wantDataDir, "rw-data", false, "alias for -rw $XDG_DATA_HOME (fallback to ~/.local/share/)")
	flag.Var(&envs, "env", "set environment variable")
	flag.Parse()

	entryPoint := flag.Args()

	if chdir == "." {
		chdir = cwd
	}

	args := []string{
		"bwrap",

		// common stuff we need
		"--ro-bind-try", "/usr", "/usr",
		"--ro-bind-try", "/bin", "/bin",
		"--ro-bind-try", "/lib", "/lib",
		"--ro-bind-try", "/lib64", "/lib64",
		"--ro-bind-try", "/opt", "/opt",
		// whoami needs /etc/group and /etc/passwd
		// we need /etc/resolve.conf for internet
		"--ro-bind-try", "/etc/", "/etc/",
	}

	if wantConfig {
		configDir, err := os.UserConfigDir()
		if err != nil {
			return err
		}
		args = append(args, "--ro-bind", configDir, configDir)
	}

	if wantCache {
		cache, err := os.UserCacheDir()
		if err != nil {
			return err
		}
		args = append(args, "--bind", cache, cache)
	}

	if wantDataDir {
		// stdlib has no os.UserDataDir()
		dataDir := os.Getenv("XDG_DATA_HOME")
		if dataDir == "" {
			home, err := os.UserHomeDir()
			if err != nil {
				return err
			}
			dataDir = filepath.Join(home, ".local", "share")
		}

		args = append(args, "--bind", dataDir, dataDir)
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

	// we could manage env variables through cmd.Env instead
	// but --clearenv sets the right working directory for us
	// AND -print shows env vars

	if clearEnv {
		args = append(args, "--clearenv")
	} else if !inheritAllEnv {
		// only inherit obvious ones
		envs := []string{
			"HOME",
			"TERM",
			"EDITOR",
			"VISUAL",
			"PATH",
			"XDG_CONFIG_HOME",
			"XDG_CACHE_HOME",
			"XDG_DATA_HOME",
			"XDG_RUNTIME_DIR",
		}
		for _, k := range envs {
			v := os.Getenv(k)
			if v != "" {
				args = append(args, "--setenv", k, v)
			}
		}

	}

	for _, e := range envs {
		keyval := strings.SplitN(e, "=", 2)
		if len(keyval) != 2 {
			return fmt.Errorf("bad environment variable %s", e)
		}
		args = append(args, "--setenv", keyval[0], keyval[1])
	}

	// used by systemd and
	// NOTE: can't play audio without it
	runtimeDir := os.Getenv("XDG_RUNTIME_DIR")
	if runtimeDir == "" { // fallback
		u, err := user.Current()
		if err != nil {
			return err
		}
		runtimeDir = filepath.Join("/run", "user", u.Uid)
	}
	args = append(args, "--bind-try", runtimeDir, runtimeDir)

	// set special file systems
	// NOTE: must be set before --ro-bind. Otherwise an --ro-bind can override
	// the permissions of say, a --dev-bind
	// had a hard-to-find bug where I couldn't use the GPU because --ro-bind / / overwrote /dev
	args = append(args, []string {
		"--tmpfs", "/tmp",
		"--proc", "/proc",

		// for hardware. GPU won't work without it
		"--dev-bind", "/dev", "/dev",
	}...)


	args = append(args, "--")
	if len(entryPoint) == 0 {
		entryPoint = []string{"sh"}
	}
	args = append(args, entryPoint...)

	if print {
		fmt.Println(strings.Join(args, " "))
		return nil
	}

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
