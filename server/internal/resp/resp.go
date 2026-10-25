// Package resp 统一 HTTP 响应 {code, msg, data}。
package resp

import (
	"errors"
	"log"
	"net/http"

	"github.com/gin-gonic/gin"

	"fitness/internal/errs"
)

type Body struct {
	Code int         `json:"code"`
	Msg  string      `json:"msg"`
	Data interface{} `json:"data"`
}

func OK(c *gin.Context, data interface{}) {
	c.JSON(http.StatusOK, Body{Code: errs.CodeOK, Msg: "ok", Data: data})
}

func Fail(c *gin.Context, err error) {
	var ae *errs.AppError
	if !errors.As(err, &ae) {
		// 非业务错误（真正的内部异常）：对外统一话术，对内必须留日志（L9）
		log.Printf("[resp] %s %s 内部错误: %v", c.Request.Method, c.Request.URL.Path, err)
		ae = errs.Wrap(errs.CodeServer, "服务器开小差了，请稍后再试", err)
	}
	status := http.StatusOK // 业务错误统一 200 + code，便于小程序端统一处理
	switch ae.Code {
	case errs.CodeUnauthorized:
		status = http.StatusUnauthorized
	case errs.CodeForbidden:
		status = http.StatusForbidden
	}
	c.JSON(status, Body{Code: ae.Code, Msg: ae.Msg})
}

// Handle 供 handler 收拢错误返回路径：`if err != nil { resp.Handle(c, err); return }`
func Handle(c *gin.Context, err error) { Fail(c, err) }
