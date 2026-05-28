# YTDL2 Windows Port

This folder contains the Windows build of the Fyne GUI app.

Expected runtime layout:

```text
win/
  ytdl2-windows.exe
  bin/
    yt-dlp.exe
    ffmpeg.exe
    ffprobe.exe
```

Build from this folder on Linux with MinGW:

```bash
GOOS=windows GOARCH=amd64 CGO_ENABLED=1 \
CC=x86_64-w64-mingw32-gcc CXX=x86_64-w64-mingw32-g++ \
GOCACHE=/tmp/go-build \
go build -buildvcs=false -ldflags -H=windowsgui -o ytdl2-windows.exe .
```

On Windows, keep the `bin` folder next to `ytdl2-windows.exe` so downloads can use the bundled `yt-dlp.exe` and `ffmpeg.exe`.
