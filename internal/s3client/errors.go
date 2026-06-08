package s3client

import (
	"context"
	"errors"
	"fmt"
	"net/http"

	"github.com/minio/minio-go/v7"
)

type S3Error struct {
	Op         string
	Key        string
	StatusCode int
	Code       string
	Message    string
	RequestID  string
	HostID     string
	Err        error
}

func (e *S3Error) Error() string {
	msg := fmt.Sprintf("%s", e.Op)
	if e.Key != "" {
		msg += fmt.Sprintf(" key=%s", e.Key)
	}
	if e.StatusCode > 0 {
		msg += fmt.Sprintf(": HTTP %d %s", e.StatusCode, http.StatusText(e.StatusCode))
	} else {
		msg += ": (No HTTP response—possibly a network error, timeout, or TLS issue)"
	}
	if e.Code != "" {
		msg += fmt.Sprintf(" code=%s", e.Code)
	}
	if e.Message != "" {
		msg += fmt.Sprintf(" message=%q", e.Message)
	}
	if e.RequestID != "" {
		msg += fmt.Sprintf(" request-id=%s", e.RequestID)
	}
	if e.HostID != "" {
		msg += fmt.Sprintf(" host-id=%s", e.HostID)
	}
	msg += fmt.Sprintf(" (%v)", e.Err)
	return msg
}

func (e *S3Error) Unwrap() error { return e.Err }

func wrapErr(op, key string, err error) error {
	if err == nil {
		return nil
	}
	if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
		return err
	}
	e := &S3Error{Op: op, Key: key, Err: err}
	resp := minio.ToErrorResponse(err)
	e.StatusCode = resp.StatusCode
	e.Code = resp.Code
	e.Message = resp.Message
	e.RequestID = resp.RequestID
	e.HostID = resp.HostID
	return e
}
