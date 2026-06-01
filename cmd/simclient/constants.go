package main

import "time"

const (
	defaultAPIBase       = "http://localhost:8080"
	defaultViewerBase    = "http://localhost:8080"
	defaultChunks        = 12
	defaultInterval      = 5 * time.Second
	defaultMediaType     = "audio"
	defaultChunkSize     = "64KiB"
	defaultCheckinEvery  = 3
	clientRequestTimeout = 30 * time.Second
	chunkDuration        = 10 * time.Second
)

const (
	desktopSourceGenerated = "generated"
	desktopSourceFiles     = "files"
	desktopSourceFFmpeg    = "ffmpeg"
	defaultDesktopSource   = desktopSourceGenerated
)

const (
	defaultDesktopMaxAttempts = 5
	defaultDesktopRetryDelay  = time.Second
	defaultFFmpegInputFormat  = "lavfi"
	defaultFFmpegInput        = "testsrc2=size=1280x720:rate=15"
	defaultFFmpegVideoCodec   = "mpeg4"
	defaultFFmpegDuration     = 15 * time.Second
	defaultFFmpegSegmentTime  = 5 * time.Second
)
