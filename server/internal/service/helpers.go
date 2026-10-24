package service

import "fitness/internal/errs"

// BadParamJSON 请求体 JSON 解析失败的统一话术
func BadParamJSON() error { return errs.BadParam("请求参数格式有误") }

// NotFoundJSON 资源不存在
func NotFoundJSON() error { return errs.NotFound("资源不存在") }

// BadParamMsg 常规参数错误（handler 内联校验用）
func BadParamMsg(msg string) error { return errs.BadParam(msg) }
