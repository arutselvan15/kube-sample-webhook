// Package log provides logger
package log

import (
	cl "github.com/arutselvan15/estore-common/log"
	gLog "github.com/arutselvan15/go-utils/log"
)

var (
	logInstance gLog.CommonLog
)

// GetLogger the Log object
func GetLogger() gLog.CommonLog {
	if logInstance == nil {
		logInstance = cl.GetLogger("product").SetComponent("webhook")
	}

	return logInstance
}
