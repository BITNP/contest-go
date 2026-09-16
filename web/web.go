package web

import "embed"

// FS 内嵌前端模板与静态资源，运行时不依赖外部构建产物。
//
// templates/  Go html/template 页面模板
// static/     直接通过 /static/ 提供的 css/js/img
//
// 前端无构建步骤：static 下的文件即源码，模板直接引用。
//
//go:embed templates static
var FS embed.FS
