package logger

import (
	"encoding/json"
	"errors"
	"fmt"
	"github.com/valyala/fasthttp"
	coreLog "log"
	"os"
	"time"
)

type severity string

const (
	CRITICAL severity = "CRITICAL"
	DEBUG    severity = "DEBUG"
	ERROR    severity = "ERROR"
	INFO     severity = "INFO"
	WARNING  severity = "WARNING"
)

type logEntry struct {
	Severity severity    `json:"severity"`
	Tags     []string    `json:"tags"`
	Message  string      `json:"message"`
	Data     interface{} `json:"data"`
}

type Options struct {
	Host   string
	System string
	Token  string
}

type logger struct {
	options Options
}

const format = "2006-01-02 15:04:05"

var logr *logger

func Init(o Options) {
	if logr != nil {
		Error([]string{"logging"}, "Trying to instantiate an already instantiated logger", nil)
		return
	}

	logr = &logger{o}
}

func Critical(tags []string, message string, data interface{}) {
	go log(newEntry(CRITICAL, tags, message, data))
}

func Debug(tags []string, message string, data interface{}) {
	go log(newEntry(DEBUG, tags, message, data))
}

func Error(tags []string, message string, data interface{}) {
	go log(newEntry(ERROR, tags, message, data))
}

func Fatal(tags []string, message string, data interface{}) {
	e := newEntry(CRITICAL, tags, message, data)
	if err := log(e); err == nil {
		writeLocalLog(e)
	}
	os.Exit(1)
}

func Info(tags []string, message string, data interface{}) {
	go log(newEntry(INFO, tags, message, data))
}

func Warning(tags []string, message string, data interface{}) {
	go log(newEntry(WARNING, tags, message, data))
}

func newEntry(severity severity, tags []string, message string, data interface{}) logEntry {
	return logEntry{
		severity,
		append([]string{logr.options.System}, tags...),
		message,
		data,
	}
}

func log(e logEntry) error {
	if logr == nil {
		coreLog.Fatal("You need to instantiate the logger first")
	}

	body, err := json.Marshal(e)
	if err != nil {
		writeLocalLog(e)
		Error([]string{"logging"}, fmt.Sprintf("Could not post to log due to \"data\" wasn't encodable - See local log"), "")
	}

	if err == nil {
		err = postLog(body)
		if err != nil {
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

	err := client.Do(req, res)
	return err
}

func writeLocalLog(e logEntry) {
	t := time.Now()
	ts := t.Format(format)

	fmt.Printf("%v %v - %v - %v", ts, e.Severity, e.Tags, e.Message)
}
