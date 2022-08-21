# rclone-autosync

Small utility using [Rclone](https://rclone.org/) to automatically sync a local directory with a remote. I personally
use it to sync Google Drive on Fedora, because Google has not released an official client for Drive
[yet](https://abevoelker.github.io/how-long-since-google-said-a-google-drive-linux-client-is-coming/).

**How does it work?**

- It periodically runs `rclone sync remote_name:remote_path local_path` to fetch remote changes,
- It polls the local filesystem to detect any change and runs `rclone sync local_path remote_name:remote_path` when
  needed.

Not the most efficient implementation, but it is quite robust and fits my needs.

## Usage

**Install Rclone**

Follow the [official instructions](https://rclone.org/install).

**Install `rclone-autosync`**

```
go install github.com/jmichiels/rclone-autosync/cmd/rclone-autosync@latest
```

**Configure a [systemd](https://www.freedesktop.org/wiki/Software/systemd/) service**

Create a new systemd user service file in `~/.config/systemd/user/rclone-autosync.service`. Fix the paths as required.

```
[Unit]
Description=rclone-autosync service
Wants=network.target
After=network.target

[Service]
ExecStart=/home/user/go/bin/rclone-autosync --rclone /path/to/rclone remote-name:/remote/path/ /local/path/
KillSignal=SIGINT

[Install]
WantedBy=default.target
```

Enable and start the service:

```
systemctl --user enable rclone-autosync
systemctl --user start rclone-autosync
```

Check its status:

```
systemctl --user status rclone-autosync
```
