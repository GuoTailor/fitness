// Package errs 统一业务错误。handler 层只需 return errs.Xxx / errs.New(...)，
// 由 resp 包装为 {code, msg, data} HTTP 响应。
package errs

import "fmt"

const (
	CodeOK          = 0
	CodeBadParam    = 40001
	CodeUnauthorized = 40101
	CodeForbidden   = 40301
	CodeNotFound    = 40401
	CodeConflict    = 40901 // 并发冲突/重复操作
	CodeBiz         = 42201 // 常规业务校验失败
	CodeServer      = 50001
)

type AppError struct {
	Code int    `json:"code"`
	Msg  string `json:"msg"`
	Err  error  `json:"-"`
}

func (e *AppError) Error() string {
	if e.Err != nil {
		return fmt.Sprintf("code=%d msg=%s err=%v", e.Code, e.Msg, e.Err)
	}
	return fmt.Sprintf("code=%d msg=%s", e.Code, e.Msg)
}

func (e *AppError) Unwrap() error { return e.Err }

func New(code int, msg string) *AppError           { return &AppError{Code: code, Msg: msg} }
func Wrap(code int, msg string, err error) *AppError { return &AppError{Code: code, Msg: msg, Err: err} }

func BadParam(msg string) error         { return New(CodeBadParam, msg) }
func Unauthorized(msg string) error     { return New(CodeUnauthorized, msg) }
func Forbidden(msg string) error        { return New(CodeForbidden, msg) }
func NotFound(msg string) error         { return New(CodeNotFound, msg) }
func Conflict(msg string) error         { return New(CodeConflict, msg) }
func Biz(msg string) error              { return New(CodeBiz, msg) }
func Server(err error, msg string) error { return Wrap(CodeServer, msg, err) }
