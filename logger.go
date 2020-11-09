package logger

import (
	"encoding/json"
	"errors"
	"fmt"
	coreLog "log"
	"os"
	"time"

	"github.com/valyala/fasthttp"
)

type severity string

const (
	criticalSeverity severity = "CRITICAL"
	debugSeverity    severity = "DEBUG"
	errorSeverity    severity = "ERROR"
	infoSeverity     severity = "INFO"
	warningSeverity  severity = "WARNING"
)

type logEntry struct {
	Severity severity    `json:"severity"`
	Tags     []string    `json:"tags"`
	Message  string      `json:"message"`
	Data     interface{} `json:"data"`
}

// Options is the config that is used for bootstrapping the logger.
// Default is posting logs to remote server but omitting host will
// write local logs instead.
type Options struct {
	Host    string `json:"host"`    // When omitting, logs will be written locally
	System  string `json:"system"`  // Required
	Token   string `json:"token"`   // Required if Host is set
	Local   bool   `json:"local"`   // Default false - If you want to force local logs in addition to the remote ones
	Timeout int    `json:"timeout"` // Default 10 - How long communication with server is allowed to take before giving up and writing a local log
}

type logger struct {
	options Options
}

const format = "2006-01-02 15:04:05"
const timeout = 10

var logTags = []string{"logging"}

var logr *logger

// Init bootstraps the config to the logger instance
func Init(o Options) error {
	if o.Timeout < 1 {
		o.Timeout = timeout
	}

	if logr != nil {
		err := errors.New("Trying to instantiate an already instantiated logger")
		Error(logTags, err.Error(), nil)
		return err
	}

	logr = &logger{o}

	if o.Host == "" {
		Warning(logTags, "Host is not set", nil)
	}

	return nil
}

// Critical creates a log for critical error messages.
// Is synchronous and if you need concurrency run it as a goroutine.
func Critical(tags []string, message string, data interface{}) {
	log(newEntry(criticalSeverity, tags, message, data))
}

// Debug creates a log for debug messages.
// Is synchronous and if you need concurrency run it as a goroutine.
func Debug(tags []string, message string, data interface{}) {
	log(newEntry(debugSeverity, tags, message, data))
}

// Error creates a log for error messages.
// Is synchronous and if you need concurrency run it as a goroutine.
func Error(tags []string, message string, data interface{}) {
	log(newEntry(errorSeverity, tags, message, data))
}

// Fatal creates a log for critical error messages and shuts down the server.
// Is synchronous and should not be ran concurrently as it would defeat the
// purpose of being a fatal action.
func Fatal(tags []string, message string, data interface{}) {
	e := newEntry(criticalSeverity, tags, message, data)
	if err := log(e); err == nil && !logr.options.Local {
		// If an error didnt occur here, it wont write a local log so we do it here
		writeLocalLog(e)
	}
	os.Exit(1)
}

// Info creates a log for informational messages.
// Is synchronous and if you need concurrency run it as a goroutine.
func Info(tags []string, message string, data interface{}) {
	log(newEntry(infoSeverity, tags, message, data))
}

// Warning creates a log for warning messages.
// Is synchronous and if you need concurrency run it as a goroutine.
func Warning(tags []string, message string, data interface{}) {
	log(newEntry(warningSeverity, tags, message, data))
}

func newEntry(severity severity, tags []string, message string, data interface{}) logEntry {
	if logr == nil {
		coreLog.Fatal("You need to instantiate the logger first")
	}
	return logEntry{
		severity,
		append([]string{logr.options.System}, tags...),
		message,
		data,
	}
}

func log(e logEntry) error {
	body, err := json.Marshal(e)
	if err != nil {
		writeLocalLog(e)
		Error(logTags, fmt.Sprintf("Could not post to log due to \"data\" wasn't encodable - See local log"), "")
	}

	if err == nil {
		err = postLog(body)
		if err != nil || logr.options.Local {
			writeLocalLog(e)
		}
		return err
	}

	return nil
}

func postLog(body []byte) error {
	if logr.options.Host == "" {
		return errors.New("Host is not set")
	}

	req := fasthttp.AcquireRequest()
	req.SetRequestURI(logr.options.Host)
	req.Header.SetMethod("POST")
	req.Header.SetContentType("application/json")
	req.Header.Add("billes-log-token", logr.options.Token)
	req.SetBody(body)
	res := fasthttp.AcquireResponse()
	client := &fasthttp.Client{}

	err := client.DoTimeout(req, res, time.Duration(logr.options.Timeout)*time.Second)
	if err != nil && logr.options.Host != "" {
		entry := newEntry(warningSeverity, logTags, fmt.Sprintf("Failed while sending log entry request: %s", err.Error()), nil)
		writeLocalLog(entry)
	}
	return err
}

func writeLocalLog(e logEntry) {
	t := time.Now()
	ts := t.Format(format)

	fmt.Printf("%v %v - %v - %v\n", ts, e.Severity, e.Tags, e.Message)
}
