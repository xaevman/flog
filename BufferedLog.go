//  ---------------------------------------------------------------------------
//
//  BufferedLog.go
//
//  Copyright (c) 2014, Jared Chavez. 
//  All rights reserved.
//
//  Use of this source code is governed by a BSD-style
//  license that can be found in the LICENSE file.
//
//  -----------

package flog

import (
    "bytes"
    "io"
    "log"
    "os"
    "sync"
    "sync/atomic"
    "time"
)

// BufferedLog represents a buffered, file-backed logger and enforces a standardized
// logging format. New logging entries are sent to a memory buffer and
// periodically flushed to the backing file at configurable intervals 
// by a seperate goroutine.
type BufferedLog struct {
    baseDir  string
    buffer   bytes.Buffer
    chClose  chan interface{}
    enabled  int32
    file     *os.File
    flushSec int32
    lock     sync.RWMutex
    logger   *log.Logger
    name     string
}

// BaseDir returns the base directory of the file backing this BufferedLog instance.
func (this *BufferedLog) BaseDir() string {
    this.lock.RLock()
    defer this.lock.RUnlock()

    return this.baseDir
}

// Close disables the BufferedLog instance, flushes any remaining entries to disk, and
// then closes the backing log file.
func (this *BufferedLog) Close() {
    this.lock.Lock()
    defer this.lock.Unlock()

    this.enabled = 0

    this.print("==== Close log ====")

    // stop flush routine
    this.chClose <- nil
    <-this.chClose

    // flush logs
    this.flushLogs()

    // close file
    this.file.Close()
}

// Disable temporarily disables the BufferedLog instance. New calls to Print will have no
// effect.
func (this *BufferedLog) Disable() {
    atomic.StoreInt32(&this.enabled, 0)
}

// Enable re-enables an BufferedLog instance.
func (this *BufferedLog) Enable() {
    atomic.StoreInt32(&this.enabled, 1)
}

// FlushInterval returns the interval between log flushes in seconds.
func (this *BufferedLog) FlushIntervalSec() int32 {
    return atomic.LoadInt32(&this.flushSec)
}

// Name returns the friendly name of the log. 
func (this *BufferedLog) Name() string {
    this.lock.RLock()
    defer this.lock.RUnlock()

    return this.name
}

// Print formats and buffers a new log entry as long as the BufferedLog instance
// is enabled.
func (this *BufferedLog) Print(format string, v ...interface{}) {
    this.lock.RLock()
    defer this.lock.RUnlock()

    if atomic.LoadInt32(&this.enabled) < 1 {
        return
    }

    if v == nil {
        this.print(format)
    } else {
        this.print(format, v...)
    }
}

// SetFlushIntervalSec sets the interval at which the log buffer worker
// will attempt to flush new entries into the backing log file.
func (this *BufferedLog) SetFlushIntervalSec(interval int32) {
    atomic.StoreInt32(&this.flushSec, interval)
}

// asyncFlush is run in a separate goroutine and periodically flushes 
// buffered entries to the backing file.
func (this *BufferedLog) asyncFlush() {
    run := true

    for run {
        flushSec := atomic.LoadInt32(&this.flushSec)

        select {
            case <-this.chClose:
                run = false
                this.print("Async log shutdown")
                continue
            case <-time.After(time.Duration(flushSec) * time.Second):
                this.flushLogs()
        }
    }

    this.chClose <- nil
}

// flushLogs copies the contents of the log buffer into the open log file.
func (this *BufferedLog) flushLogs() {
    _, err := io.Copy(this.file, &this.buffer)
    if err != nil {
        panic(err)
    }

    err = this.file.Sync()
    if err != nil {
        panic(err)
    }
}

// print performs log entry formatting and uses a std library logger to
// write the formatted entry into the BufferedLog's buffer.
func (this *BufferedLog) print(format string, v ...interface{}) {
    format = fixFormat(this.name, format)

    if v == nil {
        log.Print(format)
        this.logger.Print(format)
    } else {
        log.Printf(format, v...)
        this.logger.Printf(format, v...)
    }
}


