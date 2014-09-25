//  ---------------------------------------------------------------------------
//
//  init.go
//
//  Copyright (c) 2014, Jared Chavez. 
//  All rights reserved.
//
//  Use of this source code is governed by a BSD-style
//  license that can be found in the LICENSE file.
//
//  -----------

// Package flog provides facilities for using and managing 
// file-backed logger objects.
package flog

import (
    "fmt"
    "log"
    "os"
    "path"
    "strings"
    "time"
)

// Default flush interval, in seconds, for BufferedLog instances.
const DefaultFlushIntervalSec = 5

// Logger format flags.
const FLogFlags = log.Ldate | log.Lmicroseconds | log.Lshortfile

// Log file open flags.
const FLogOpenFlags = os.O_RDWR | os.O_APPEND | os.O_CREATE

// Enumeration of different FLog implementations.
const (
    BufferedFile = iota
    DirectFile
)

// FLog provides a common interface for different file-backed logs. This package
// includes two primary implementations; BufferedLog and DirectLog.
type FLog interface {
    BaseDir() string
    Close()
    Disable()
    Enable()
    Name() string
    Print(format string, v ...interface{})
}

// New returns a new FLog instance of the requested type. The backing log file is 
// created or opened for append.
func New(name, logPath string, logType int) FLog {
    var newLog FLog

    mkdir(logPath)

    f, err := os.OpenFile(
        path.Join(logPath, name + ".log"), 
        FLogOpenFlags, 
        0660,
    )
    if err != nil {
        return nil
    }

    switch logType {
    case BufferedFile:

        bLog := BufferedLog {
            baseDir  : logPath,
            chClose  : make(chan interface{}, 0),
            enabled  : 1,
            flushSec : DefaultFlushIntervalSec,
            name     : name,
        }

        bLog.file = f

        l := log.New(&bLog.buffer, "", FLogFlags)
        bLog.logger = l

        go bLog.asyncFlush()

        newLog = &bLog
        break
    
    case DirectFile:

        dLog := DirectLog {
            baseDir : logPath,
            enabled : 1,
            name    : name,
        }

        dLog.file = f

        l := log.New(dLog.file, "", FLogFlags)
        dLog.logger = l

        newLog = &dLog
        break
    }

    newLog.Print("==== Log init ====")

    return newLog
}

// Rotate takes a given FLog instance, closes it, timestamps and moves the 
// backing log file into an old subdirectory, before opening and returning a new
// FLog instance at the original location.
func Rotate(log FLog) FLog {
    log.Close()

    mkPath := path.Join(log.BaseDir(), "old")

    mkdir(mkPath)

    now     := time.Now()
    newPath := path.Join(
        mkPath, 
        fmt.Sprintf(
            "%d%d%d-%s.log", 
            now.Year(), 
            now.Month(), 
            now.Day(), 
            log.Name(),
        ),
    )
    oldPath := path.Join(
        log.BaseDir(), 
        log.Name() + ".log",
    )

    err := os.Rename(
        oldPath, 
        newPath,
    )

    if err != nil {
        panic(err)
    }

    var newLog FLog
    bLog, ok := log.(*BufferedLog)

    if ok {
        newLog = New(log.Name(), log.BaseDir(), BufferedFile)
        newLog.(*BufferedLog).SetFlushIntervalSec(bLog.FlushIntervalSec())
    } else {
        newLog = New(log.Name(), log.BaseDir(), DirectFile)
    }

    return newLog
}

// fixFormat takes a given format string, prepends the log name to the beginning of
// the string, and makes sure that it is terminated with a newline. The processed
// string is then returned to the caller.
func fixFormat(name, format string) string {
    if format[len(format) - 1] == '\n' {
        return fmt.Sprintf(
            "[%s] %s",
            strings.ToUpper(name),
            format,
        )
    }

    return fmt.Sprintf(
        "[%s] %s\n",
        strings.ToUpper(name),
        format,
    )
}

// init sets the default Logger flags to match the FLog packages preferred flags.
func init() {
    log.SetFlags(FLogFlags)
}

// mkdir wraps os.MkdirAll with default privs of 770 and panics on errors.
func mkdir(path string) {
    err := os.MkdirAll(path, 0770)
    if err != nil {
        panic(err)
    }
}
