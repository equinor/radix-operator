package logs

import (
	"errors"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/equinor/radix-common/utils"
)

// GetLogParams Gets parameters for a log output
func GetLogParams(r *http.Request) (since time.Time, asFile bool, asFollow bool, logLines *int64, err error, previousLog bool) {
	sinceTime := r.FormValue("sinceTime")
	lines := r.FormValue("lines")
	file := r.FormValue("file")
	follow := r.FormValue("follow")
	previous := r.FormValue("previous")
	var errs []error

	if !strings.EqualFold(strings.TrimSpace(sinceTime), "") {
		var err error
		since, err = utils.ParseTimestamp(sinceTime)
		if err != nil {
			errs = append(errs, err)
		}
	}
	if strings.TrimSpace(file) != "" {
		var err error
		asFile, err = strconv.ParseBool(file)
		if err != nil {
			errs = append(errs, err)
		}
	}
	if strings.TrimSpace(follow) != "" {
		var err error
		asFollow, err = strconv.ParseBool(follow)
		if err != nil {
			errs = append(errs, err)
		}
	}
	if strings.TrimSpace(previous) != "" {
		var err error
		previousLog, err = strconv.ParseBool(previous)
		if err != nil {
			errs = append(errs, err)
		}
	}
	if strings.TrimSpace(lines) != "" {
		var err error
		val, err := strconv.ParseInt(lines, 10, 64)
		if err != nil {
			errs = append(errs, err)
		}
		logLines = &val
	}
	return since, asFile, asFollow, logLines, errors.Join(errs...), previousLog
}
