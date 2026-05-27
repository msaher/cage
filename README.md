# cage

Run processes in a sandbox. Full access to your system's binaries and tools, but the process can't mess anything up. It's like docker but without the pain.

Powered by [bubblewrap](https://github.com/containers/bubblewrap).

## Install

```
go install github.com/msaher/cage/cage@latest
```

Requires `bwrap` to be installed on your system.

## Usage

```
cage [flags] <command>
```

By default, system files are mounted as read-only. The process can use all your tools and binaries but can't write anywhere unless you explicitly allow it

```sh
# run a shell in a sandbox
cage

# run a script, read-only
cage ./script.sh

# allow writes to current directory only
cage -rw . ./script.sh

# no network access
cage -offline ./script.sh

# expose a specific dir read-only
cage -ro ~/projects/mylib make
```

## Flags

| Flag | Description |
|------|-------------|
| `-rw <dir>` | Writable directory (repeatable) |
| `-ro <dir>` | Read-only directory (repeatable) |
| `-chdir <dir>` | Working directory inside sandbox |
| `-offline` | No network access |
| `-clearenv` | Clear all environment variables |
| `-inherit-all-env` | Inherit all host environment variables |
| `-env KEY=VALUE` | Set an environment variable (repeatable) |
| `-env-file <file>` | Load environment variables from file |
| `-ro-config` | Expose `$XDG_CONFIG_HOME` read-only |
| `-rw-cache` | Expose `$XDG_CACHE_HOME` read-write |
| `-rw-data` | Expose `$XDG_DATA_HOME` read-write |
| `-print` | Print bwrap command without running |

## Examples

### Make your entire system read-only

```sh
cage -ro / bash
```

### Run an AI agent

```sh
alias aicage="cage -rw-cache -rw-data -ro-config -rw ."
aicage my-agent
```

### Run a random script from the internet

```sh
cage -offline bash install.sh
```

### Play media

```sh
cage -ro ~/media mpv video.mkv
```

### Contain a virus

```sh
cage -ro . ./virus.sh
# Running virus...
# About to ruin your system muhahahahahah
# rm: cannot remove 'life-savings.txt': Read-only file system
# Virus failed?! Impossible! How?!! User must've used cage grrrr
```

### Fearlessly run anything

```sh
cage rm -r mydir
# rm: cannot remove 'mydir': Read-only file system
```

## How it works

cage is a thin wrapper around `bwrap`. It mounts the host filesystem read-only by default, then selectively opens up paths you specify. `/dev` is bind-mounted so GPU, audio, and hardware work normally.
